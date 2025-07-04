package sync

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	versioned2 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	creationTimestamp = "{.metadata.creationTimestamp}"
)

type QueueManager struct {
	queueMap map[string]Semaphore
	lock     *sync.Mutex
	logger   *zap.SugaredLogger
}

func NewQueueManager(logger *zap.SugaredLogger) *QueueManager {
	return &QueueManager{
		queueMap: make(map[string]Semaphore),
		lock:     &sync.Mutex{},
		logger:   logger,
	}
}

// getSemaphore returns existing semaphore created for repository or create
// a new one with limit provided in repository
// Semaphore: nothing but a waiting and a running queue for a repository
// with limit deciding how many should be running at a time.
func (qm *QueueManager) getSemaphore(repo *v1alpha1.Repository) (Semaphore, error) {
	repoKey := RepoKey(repo)

	if sema, found := qm.queueMap[repoKey]; found {
		if err := qm.checkAndUpdateSemaphoreSize(repo, sema); err != nil {
			return nil, err
		}
		return sema, nil
	}

	// create a new semaphore; can't assume callers have checked that ConcurrencyLimit is set
	limit := 0
	if repo.Spec.ConcurrencyLimit != nil {
		limit = *repo.Spec.ConcurrencyLimit
	}
	qm.queueMap[repoKey] = newSemaphore(repoKey, limit)

	return qm.queueMap[repoKey], nil
}

func (qm *QueueManager) checkAndUpdateSemaphoreSize(repo *v1alpha1.Repository, semaphore Semaphore) error {
	limit := *repo.Spec.ConcurrencyLimit
	if limit != semaphore.getLimit() {
		if semaphore.resize(limit) {
			return nil
		}
		return fmt.Errorf("failed to resize semaphore")
	}
	return nil
}

// AddListToRunningQueue adds the pipelineRun to the waiting queue of the repository
// and if it is at the top and ready to run which means currently running pipelineRun < limit
// then move it to running queue
// This adds the pipelineRuns in the same order as in the list.
func (qm *QueueManager) AddListToRunningQueue(repo *v1alpha1.Repository, list []string) ([]string, error) {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	sema, err := qm.getSemaphore(repo)
	if err != nil {
		return []string{}, err
	}

	for _, pr := range list {
		if sema.addToQueue(pr, time.Now()) {
			qm.logger.Infof("added pipelineRun (%s) to running queue for repository (%s)", pr, RepoKey(repo))
		}
	}

	// it is possible something besides PAC set the PipelineRun to Pending; if concurrency limit has not
	// been set, return all the pending PipelineRuns; also, if the limit is zero, that also means do not throttle,
	// so we return all the PipelinesRuns, the for loop below skips that case as well
	if repo.Spec.ConcurrencyLimit == nil || *repo.Spec.ConcurrencyLimit == 0 {
		return sema.getCurrentPending(), nil
	}

	acquiredList := []string{}
	for i := 0; i < *repo.Spec.ConcurrencyLimit; i++ {
		acquired := sema.acquireLatest()
		if acquired != "" {
			qm.logger.Infof("moved (%s) to running for repository (%s)", acquired, RepoKey(repo))
			acquiredList = append(acquiredList, acquired)
		}
	}

	return acquiredList, nil
}

func (qm *QueueManager) AddToPendingQueue(repo *v1alpha1.Repository, list []string) error {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	sema, err := qm.getSemaphore(repo)
	if err != nil {
		return err
	}

	for _, pr := range list {
		if sema.addToPendingQueue(pr, time.Now()) {
			qm.logger.Infof("added pipelineRun (%s) to pending queue for repository (%s)", pr, RepoKey(repo))
		}
	}
	return nil
}

func (qm *QueueManager) RemoveFromQueue(repoKey, prKey string) bool {
	qm.lock.Lock()
	sema, found := qm.queueMap[repoKey]
	qm.lock.Unlock()

	if !found {
		return false
	}

	// First check if the PipelineRun is in the pending queue
	pending := sema.getCurrentPending()
	wasPending := false
	for _, p := range pending {
		if p == prKey {
			wasPending = true
			break
		}
	}

	// Remove from queue (works for both pending and running)
	sema.removeFromQueue(prKey)

	// Try to release - will only succeed if it was running
	released := sema.release(prKey)
	if released {
		qm.logger.Infof("removed running PipelineRun (%s) for repository (%s)", prKey, repoKey)
	} else if wasPending {
		qm.logger.Infof("removed pending PipelineRun (%s) for repository (%s)", prKey, repoKey)
	}

	return released || wasPending
}

func (qm *QueueManager) RemoveAndTakeItemFromQueue(repo *v1alpha1.Repository, run *tektonv1.PipelineRun) string {
	repoKey := RepoKey(repo)
	prKey := PrKey(run)
	if !qm.RemoveFromQueue(repoKey, prKey) {
		return ""
	}
	sema, found := qm.queueMap[repoKey]
	if !found {
		return ""
	}

	if next := sema.acquireLatest(); next != "" {
		qm.logger.Infof("moved (%s) to running for repository (%s)", next, repoKey)
		return next
	}
	return ""
}

// FilterPipelineRunByInProgress filters the given list of PipelineRun names to only include those
// that are in a "queued" state and have a pending status. It retrieves the PipelineRun objects
// from the Tekton API and checks their annotations and status to determine if they should be included.
//
// Returns A list of PipelineRun names that are in a "queued" state and have a pending status.
func FilterPipelineRunByState(ctx context.Context, tekton versioned2.Interface, orderList []string, wantedStatus, wantedState string) []string {
	orderedList := []string{}
	for _, prName := range orderList {
		prKey := strings.Split(prName, "/")
		pr, err := tekton.TektonV1().PipelineRuns(prKey[0]).Get(ctx, prKey[1], v1.GetOptions{})
		if err != nil {
			continue
		}

		state, exist := pr.GetAnnotations()[keys.State]
		if !exist {
			continue
		}

		if state == wantedState {
			if wantedStatus != "" && pr.Spec.Status != tektonv1.PipelineRunSpecStatus(wantedStatus) {
				continue
			}
			orderedList = append(orderedList, prName)
		}
	}
	return orderedList
}

// InitQueues rebuild all the queues for all repository if concurrency is defined before
// reconciler started reconciling them.
func (qm *QueueManager) InitQueues(ctx context.Context, tekton versioned2.Interface, pac versioned.Interface) error {
	// fetch all repos
	repos, err := pac.PipelinesascodeV1alpha1().Repositories("").List(ctx, v1.ListOptions{})
	if err != nil {
		return err
	}

	// pipelineRuns from the namespace where repository is present
	// those are required for creating queues
	for _, repo := range repos.Items {
		if repo.Spec.ConcurrencyLimit == nil || *repo.Spec.ConcurrencyLimit == 0 {
			continue
		}

		qm.logger.Infof("Initializing queue for repository %s/%s with concurrency limit %d",
			repo.Namespace, repo.Name, *repo.Spec.ConcurrencyLimit)

		// add all pipelineRuns in started state to running queue
		// CRITICAL: Only get PipelineRuns that belong to this PAC repository
		prs, err := tekton.TektonV1().PipelineRuns(repo.Namespace).
			List(ctx, v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
					keys.State, kubeinteraction.StateStarted,
					keys.Repository, formatting.CleanValueKubernetes(repo.Name)),
			})
		if err != nil {
			qm.logger.Errorf("Failed to list started PipelineRuns for repo %s/%s: %v", repo.Namespace, repo.Name, err)
			continue // Continue with other repos instead of failing completely
		}

		// sort the pipelinerun by creation time before adding to queue
		sortedPRs := sortPipelineRunsByCreationTimestamp(prs.Items)

		recoveryErrors := []string{}
		for _, pr := range sortedPRs {
			// Additional validation: ensure this PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				qm.logger.Debugf("Skipping non-PAC PipelineRun %s/%s", pr.Namespace, pr.Name)
				continue
			}

			order, exist := pr.GetAnnotations()[keys.ExecutionOrder]
			if !exist {
				recoveryErrors = append(recoveryErrors,
					fmt.Sprintf("PipelineRun %s/%s missing execution_order annotation", pr.Namespace, pr.Name))
				continue // Skip this PipelineRun but continue with others
			}

			orderedList := FilterPipelineRunByState(ctx, tekton, strings.Split(order, ","), "", kubeinteraction.StateStarted)
			if len(orderedList) == 0 {
				qm.logger.Warnf("No valid PipelineRuns found in execution order for %s/%s", pr.Namespace, pr.Name)
				continue
			}

			_, err = qm.AddListToRunningQueue(&repo, orderedList)
			if err != nil {
				recoveryErrors = append(recoveryErrors,
					fmt.Sprintf("Failed to add PipelineRun %s/%s to running queue: %v", pr.Namespace, pr.Name, err))
			}
		}

		// now fetch all queued pipelineRun
		// CRITICAL: Only get PipelineRuns that belong to this PAC repository
		prs, err = tekton.TektonV1().PipelineRuns(repo.Namespace).
			List(ctx, v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
					keys.State, kubeinteraction.StateQueued,
					keys.Repository, formatting.CleanValueKubernetes(repo.Name)),
			})
		if err != nil {
			qm.logger.Errorf("Failed to list queued PipelineRuns for repo %s/%s: %v", repo.Namespace, repo.Name, err)
			continue
		}

		// sort the pipelinerun by creation time before adding to queue
		sortedPRs = sortPipelineRunsByCreationTimestamp(prs.Items)

		for _, pr := range sortedPRs {
			// Additional validation: ensure this PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				qm.logger.Debugf("Skipping non-PAC PipelineRun %s/%s", pr.Namespace, pr.Name)
				continue
			}

			order, exist := pr.GetAnnotations()[keys.ExecutionOrder]
			if !exist {
				recoveryErrors = append(recoveryErrors,
					fmt.Sprintf("PipelineRun %s/%s missing execution_order annotation", pr.Namespace, pr.Name))
				continue // Skip this PipelineRun but continue with others
			}

			orderedList := FilterPipelineRunByState(ctx, tekton, strings.Split(order, ","), tektonv1.PipelineRunSpecStatusPending, kubeinteraction.StateQueued)
			if len(orderedList) == 0 {
				qm.logger.Warnf("No valid PipelineRuns found in execution order for %s/%s", pr.Namespace, pr.Name)
				continue
			}

			if err := qm.AddToPendingQueue(&repo, orderedList); err != nil {
				recoveryErrors = append(recoveryErrors,
					fmt.Sprintf("Failed to add PipelineRun %s/%s to pending queue: %v", pr.Namespace, pr.Name, err))
			}
		}

		// Log recovery results
		if len(recoveryErrors) > 0 {
			qm.logger.Warnf("Queue recovery for repo %s/%s completed with %d errors: %v",
				repo.Namespace, repo.Name, len(recoveryErrors), recoveryErrors)
		} else {
			qm.logger.Infof("Queue recovery for repo %s/%s completed successfully", repo.Namespace, repo.Name)
		}

		// Log final queue state
		sema, found := qm.queueMap[RepoKey(&repo)]
		if found {
			running := sema.getCurrentRunning()
			pending := sema.getCurrentPending()
			qm.logger.Infof("Queue state for repo %s/%s: %d running, %d pending",
				repo.Namespace, repo.Name, len(running), len(pending))
		}
	}

	return nil
}

func (qm *QueueManager) RemoveRepository(repo *v1alpha1.Repository) {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	repoKey := RepoKey(repo)
	delete(qm.queueMap, repoKey)
}

func (qm *QueueManager) QueuedPipelineRuns(repo *v1alpha1.Repository) []string {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	repoKey := RepoKey(repo)
	if sema, ok := qm.queueMap[repoKey]; ok {
		return sema.getCurrentPending()
	}
	return []string{}
}

func (qm *QueueManager) RunningPipelineRuns(repo *v1alpha1.Repository) []string {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	repoKey := RepoKey(repo)
	if sema, ok := qm.queueMap[repoKey]; ok {
		return sema.getCurrentRunning()
	}
	return []string{}
}

func sortPipelineRunsByCreationTimestamp(prs []tektonv1.PipelineRun) []*tektonv1.PipelineRun {
	runTimeObj := []runtime.Object{}
	for i := range prs {
		runTimeObj = append(runTimeObj, &prs[i])
	}
	sort.ByField(creationTimestamp, runTimeObj)
	sortedPRs := []*tektonv1.PipelineRun{}
	for _, run := range runTimeObj {
		pr, _ := run.(*tektonv1.PipelineRun)
		sortedPRs = append(sortedPRs, pr)
	}
	return sortedPRs
}

// QueueValidationResult represents the result of queue validation.
type QueueValidationResult struct {
	RepositoryKey string
	IsValid       bool
	Errors        []string
	Warnings      []string
	RunningCount  int
	PendingCount  int
	ExpectedCount int
}

// isPACManagedPipelineRun validates that a PipelineRun is actually managed by PAC.
func isPACManagedPipelineRun(pr *tektonv1.PipelineRun) bool {
	// Check if it has the PAC managed-by label
	if managedBy, exists := pr.GetLabels()["app.kubernetes.io/managed-by"]; !exists || managedBy != "pipelinesascode.tekton.dev" {
		return false
	}

	// Check if it has the repository annotation
	if _, exists := pr.GetAnnotations()[keys.Repository]; !exists {
		return false
	}

	return true
}

// ValidateQueueConsistency validates that the in-memory queue state is consistent with
// the actual PipelineRun states in the cluster. This helps detect and report queue
// inconsistencies that can occur due to controller restarts or partial failures.
func (qm *QueueManager) ValidateQueueConsistency(ctx context.Context, tekton versioned2.Interface, pac versioned.Interface) ([]QueueValidationResult, error) {
	// Get all repositories with concurrency limits
	repos, err := pac.PipelinesascodeV1alpha1().Repositories("").List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	results := []QueueValidationResult{}

	for _, repo := range repos.Items {
		if repo.Spec.ConcurrencyLimit == nil || *repo.Spec.ConcurrencyLimit == 0 {
			continue
		}

		repoKey := RepoKey(&repo)
		result := QueueValidationResult{
			RepositoryKey: repoKey,
			IsValid:       true,
		}

		// Get current queue state with minimal lock time
		qm.lock.Lock()
		sema, found := qm.queueMap[repoKey]
		qm.lock.Unlock()

		if !found {
			result.Errors = append(result.Errors, "No queue found for repository")
			result.IsValid = false
			results = append(results, result)
			continue
		}

		// Get queue state - semaphore has its own locks
		runningInQueue := sema.getCurrentRunning()
		pendingInQueue := sema.getCurrentPending()

		result.RunningCount = len(runningInQueue)
		result.PendingCount = len(pendingInQueue)

		// Validate running PipelineRuns
		for _, prKey := range runningInQueue {
			parts := strings.Split(prKey, "/")
			if len(parts) != 2 {
				result.Errors = append(result.Errors, fmt.Sprintf("Invalid PipelineRun key format: %s", prKey))
				result.IsValid = false
				continue
			}

			pr, err := tekton.TektonV1().PipelineRuns(parts[0]).Get(ctx, parts[1], v1.GetOptions{})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("PipelineRun %s not found in cluster: %v", prKey, err))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				result.Errors = append(result.Errors, fmt.Sprintf("PipelineRun %s is not managed by PAC", prKey))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun belongs to this repository
			if repoName, exists := pr.GetAnnotations()[keys.Repository]; !exists || repoName != repo.Name {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s belongs to repository '%s' but queue is for '%s'",
						prKey, repoName, repo.Name))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun is actually in started state
			state, exists := pr.GetAnnotations()[keys.State]
			if !exists || state != kubeinteraction.StateStarted {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s in queue as running but has state '%s'", prKey, state))
				result.IsValid = false
			}

			// Check if PipelineRun is actually running (not pending)
			if pr.Spec.Status == tektonv1.PipelineRunSpecStatusPending {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s in queue as running but has pending status", prKey))
				result.IsValid = false
			}
		}

		// Validate pending PipelineRuns
		for _, prKey := range pendingInQueue {
			parts := strings.Split(prKey, "/")
			if len(parts) != 2 {
				result.Errors = append(result.Errors, fmt.Sprintf("Invalid PipelineRun key format: %s", prKey))
				result.IsValid = false
				continue
			}

			pr, err := tekton.TektonV1().PipelineRuns(parts[0]).Get(ctx, parts[1], v1.GetOptions{})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("PipelineRun %s not found in cluster: %v", prKey, err))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				result.Errors = append(result.Errors, fmt.Sprintf("PipelineRun %s is not managed by PAC", prKey))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun belongs to this repository
			if repoName, exists := pr.GetAnnotations()[keys.Repository]; !exists || repoName != repo.Name {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s belongs to repository '%s' but queue is for '%s'",
						prKey, repoName, repo.Name))
				result.IsValid = false
				continue
			}

			// Check if PipelineRun is actually in queued state
			state, exists := pr.GetAnnotations()[keys.State]
			if !exists || state != kubeinteraction.StateQueued {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s in queue as pending but has state '%s'", prKey, state))
				result.IsValid = false
			}

			// Check if PipelineRun is actually pending
			if pr.Spec.Status != tektonv1.PipelineRunSpecStatusPending {
				result.Errors = append(result.Errors,
					fmt.Sprintf("PipelineRun %s in queue as pending but has status '%s'", prKey, pr.Spec.Status))
				result.IsValid = false
			}
		}

		// Check for orphaned PipelineRuns (in cluster but not in queue)
		startedPRs, err := tekton.TektonV1().PipelineRuns(repo.Namespace).
			List(ctx, v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
					keys.State, kubeinteraction.StateStarted,
					keys.Repository, formatting.CleanValueKubernetes(repo.Name)),
			})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to list started PipelineRuns: %v", err))
		} else {
			for _, pr := range startedPRs.Items {
				// Only consider PAC-managed PipelineRuns
				if !isPACManagedPipelineRun(&pr) {
					continue
				}

				prKey := PrKey(&pr)
				found := slices.Contains(runningInQueue, prKey)
				if !found {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("PipelineRun %s in cluster as started but not in queue", prKey))
				}
			}
		}

		queuedPRs, err := tekton.TektonV1().PipelineRuns(repo.Namespace).
			List(ctx, v1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
					keys.State, kubeinteraction.StateQueued,
					keys.Repository, formatting.CleanValueKubernetes(repo.Name)),
			})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to list queued PipelineRuns: %v", err))
		} else {
			for _, pr := range queuedPRs.Items {
				// Only consider PAC-managed PipelineRuns
				if !isPACManagedPipelineRun(&pr) {
					continue
				}

				prKey := PrKey(&pr)
				found := slices.Contains(pendingInQueue, prKey)
				if !found {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("PipelineRun %s in cluster as queued but not in queue", prKey))
				}
			}
		}

		// Check concurrency limit compliance
		expectedRunning := *repo.Spec.ConcurrencyLimit
		if len(runningInQueue) > expectedRunning {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Queue has %d running PipelineRuns but limit is %d",
					len(runningInQueue), expectedRunning))
			result.IsValid = false
		}

		result.ExpectedCount = expectedRunning
		results = append(results, result)
	}

	return results, nil
}

// RepairQueue attempts to repair queue inconsistencies by removing invalid entries
// and rebuilding the queue state from the actual PipelineRun states in the cluster.
func (qm *QueueManager) RepairQueue(ctx context.Context, tekton versioned2.Interface, pac versioned.Interface) error {
	qm.logger.Info("Starting queue repair process")

	// First validate to identify issues
	validationResults, err := qm.ValidateQueueConsistency(ctx, tekton, pac)
	if err != nil {
		return fmt.Errorf("failed to validate queue consistency: %w", err)
	}

	repairCount := 0
	for _, result := range validationResults {
		if result.IsValid {
			continue
		}

		qm.logger.Infof("Repairing queue for repository %s", result.RepositoryKey)
		repoKey := result.RepositoryKey

		// Get the semaphore for this repository
		qm.lock.Lock()
		sema, found := qm.queueMap[repoKey]
		qm.lock.Unlock()

		if !found {
			qm.logger.Warnf("No queue found for repository %s, skipping repair", repoKey)
			continue
		}

		// Collect items to repair - semaphore has its own locks
		runningInQueue := sema.getCurrentRunning()
		pendingInQueue := sema.getCurrentPending()

		// Collect items to remove from running queue
		toRemoveFromRunning := []string{}
		for _, prKey := range runningInQueue {
			parts := strings.Split(prKey, "/")
			if len(parts) != 2 {
				qm.logger.Warnf("Will remove invalid PipelineRun key from running queue: %s", prKey)
				toRemoveFromRunning = append(toRemoveFromRunning, prKey)
				continue
			}

			pr, err := tekton.TektonV1().PipelineRuns(parts[0]).Get(ctx, parts[1], v1.GetOptions{})
			if err != nil {
				qm.logger.Warnf("Will remove non-existent PipelineRun from running queue: %s", prKey)
				toRemoveFromRunning = append(toRemoveFromRunning, prKey)
				continue
			}

			// Check if PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				qm.logger.Warnf("Will remove non-PAC PipelineRun from running queue: %s", prKey)
				toRemoveFromRunning = append(toRemoveFromRunning, prKey)
				continue
			}

			// Check if PipelineRun is actually in started state
			state, exists := pr.GetAnnotations()[keys.State]
			if !exists || state != kubeinteraction.StateStarted {
				qm.logger.Warnf("Will remove PipelineRun with invalid state from running queue: %s (state: %s)", prKey, state)
				toRemoveFromRunning = append(toRemoveFromRunning, prKey)
				continue
			}

			// Check if PipelineRun is actually running (not pending)
			if pr.Spec.Status == tektonv1.PipelineRunSpecStatusPending {
				qm.logger.Warnf("Will remove pending PipelineRun from running queue: %s", prKey)
				toRemoveFromRunning = append(toRemoveFromRunning, prKey)
				continue
			}
		}

		// Collect items to remove from pending queue
		toRemoveFromPending := []string{}
		for _, prKey := range pendingInQueue {
			parts := strings.Split(prKey, "/")
			if len(parts) != 2 {
				qm.logger.Warnf("Will remove invalid PipelineRun key from pending queue: %s", prKey)
				toRemoveFromPending = append(toRemoveFromPending, prKey)
				continue
			}

			pr, err := tekton.TektonV1().PipelineRuns(parts[0]).Get(ctx, parts[1], v1.GetOptions{})
			if err != nil {
				qm.logger.Warnf("Will remove non-existent PipelineRun from pending queue: %s", prKey)
				toRemoveFromPending = append(toRemoveFromPending, prKey)
				continue
			}

			// Check if PipelineRun is actually managed by PAC
			if !isPACManagedPipelineRun(pr) {
				qm.logger.Warnf("Will remove non-PAC PipelineRun from pending queue: %s", prKey)
				toRemoveFromPending = append(toRemoveFromPending, prKey)
				continue
			}

			// Check if PipelineRun is actually in queued state
			state, exists := pr.GetAnnotations()[keys.State]
			if !exists || state != kubeinteraction.StateQueued {
				qm.logger.Warnf("Will remove PipelineRun with invalid state from pending queue: %s (state: %s)", prKey, state)
				toRemoveFromPending = append(toRemoveFromPending, prKey)
				continue
			}

			// Check if PipelineRun is actually pending
			if pr.Spec.Status != tektonv1.PipelineRunSpecStatusPending {
				qm.logger.Warnf("Will remove non-pending PipelineRun from pending queue: %s (status: %s)", prKey, pr.Spec.Status)
				toRemoveFromPending = append(toRemoveFromPending, prKey)
				continue
			}
		}

		// Now perform the actual repairs
		for _, prKey := range toRemoveFromRunning {
			sema.release(prKey)
			sema.removeFromQueue(prKey)
			repairCount++
		}

		for _, prKey := range toRemoveFromPending {
			sema.removeFromQueue(prKey)
			repairCount++
		}

		// Try to acquire more PipelineRuns if we have slots available
		for {
			next := sema.acquireLatest()
			if next == "" {
				break
			}
			qm.logger.Infof("Started next PipelineRun from queue: %s", next)
		}
	}

	qm.logger.Infof("Queue repair completed. Fixed %d inconsistencies", repairCount)
	return nil
}
