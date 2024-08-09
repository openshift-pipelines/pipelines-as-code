package reconciler

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) queuePipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *tektonv1.PipelineRun) error {
	order, exist := pr.GetAnnotations()[keys.ExecutionOrder]
	if !exist {
		// if the pipelineRun doesn't have order label then wait
		return nil
	}

	repoName := pr.GetAnnotations()[keys.Repository]
	repo, err := r.repoLister.Repositories(pr.Namespace).Get(repoName)
	if err != nil {
		// if repository is not found, then skip processing the pipelineRun and return nil
		if errors.IsNotFound(err) {
			r.qm.RemoveRepository(&v1alpha1.Repository{ObjectMeta: metav1.ObjectMeta{
				Name:      repoName,
				Namespace: pr.Namespace,
			}})
			return nil
		}
		return fmt.Errorf("updateError: %w", err)
	}

	// find global repository if set
	globalRepo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(r.run.Info.Kube.Namespace).Get(
		ctx, r.run.Info.Controller.GlobalRepository, metav1.GetOptions{},
	)
	if err == nil && globalRepo != nil {
		repo.Spec.Merge(globalRepo.Spec)
	}

	// if concurrency was set and later removed or changed to zero
	// then remove pipelineRun from Queue and update pending state to running
	if repo.Spec.ConcurrencyLimit != nil && *repo.Spec.ConcurrencyLimit == 0 {
		_ = r.qm.RemoveFromQueue(repo, pr)
		if err := r.updatePipelineRunToInProgress(ctx, logger, repo, pr); err != nil {
			return fmt.Errorf("failed to update PipelineRun to in_progress: %w", err)
		}
		return nil
	}

	orderedList := strings.Split(order, ",")
	acquired, err := r.qm.AddListToQueue(repo, orderedList)
	if err != nil {
		return fmt.Errorf("failed to add to queue: %s: %w", pr.GetName(), err)
	}

	for _, prKeys := range acquired {
		nsName := strings.Split(prKeys, "/")
		pr, err = r.run.Clients.Tekton.TektonV1().PipelineRuns(nsName[0]).Get(ctx, nsName[1], metav1.GetOptions{})
		if err != nil {
			logger.Info("failed to get pr with namespace and name: ", nsName[0], nsName[1])
			// No need to return any error from here because, acquired might have more than one item
			// and if error is returned remaining items in acquired list will be left in queued state
			// and as they are popped off from semaphore in QueueManager.AddListToQueue func they won't
			// be processed in ReconcileKind func.
		}
		if err := r.updatePipelineRunToInProgress(ctx, logger, repo, pr); err != nil {
			logger.Infof("failed to update pipelineRun to in_progress: %w", err)
			// same here as above comment.
		}
	}
	return nil
}
