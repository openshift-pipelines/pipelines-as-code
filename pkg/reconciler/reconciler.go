package reconciler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const startingPipelineRunText = `Starting Pipelinerun <b>%s</b> in namespace
  <b>%s</b><br><br>You can follow the execution on the [OpenShift console](%s) pipelinerun viewer or via
  the command line with :
	<br><code>tkn pr logs -f -n %s %s</code>`

type Reconciler struct {
	run               *params.Run
	pipelineRunLister v1beta12.PipelineRunLister
	kinteract         kubeinteraction.Interface
	qm                *sync.QueueManager
}

var (
	_ pipelinerunreconciler.Interface = (*Reconciler)(nil)
	_ pipelinerunreconciler.Finalizer = (*Reconciler)(nil)
)

func (r *Reconciler) ReconcileKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// if pipelineRun is in completed state then return
	state, exist := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "state")]
	if exist && state == kubeinteraction.StateCompleted {
		return nil
	}

	// if its a GitHub App pipelineRun PR then process only if check run id is added otherwise wait
	if _, ok := pr.Annotations[filepath.Join(pipelinesascode.GroupName, "installation-id")]; ok {
		if _, ok := pr.Labels[filepath.Join(pipelinesascode.GroupName, "check-run-id")]; !ok {
			return nil
		}
	}

	if state == kubeinteraction.StateQueued {
		return r.queuePipelineRun(ctx, logger, pr)
	}

	if pr.IsDone() {
		logger = logger.With(
			"pipeline-run", pr.GetName(),
			"event-sha", pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "sha")],
		)
		logger.Infof("pipelineRun %v/%v is done, reconciling to report status!  ", pr.GetNamespace(), pr.GetName())

		provider, event, err := r.detectProvider(ctx, logger, pr)
		if err != nil {
			return fmt.Errorf("detectProvider: %w", err)
		}

		return r.reportFinalStatus(ctx, logger, event, pr, provider)
	}
	return nil
}

func (r *Reconciler) queuePipelineRun(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun) error {
	repoName := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "repository")]
	repo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().
		Repositories(pr.Namespace).Get(ctx, repoName, metav1.GetOptions{})
	if err != nil {
		// if repository is not found, then skip processing the pipelineRun and return nil
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("updateError: %w", err)
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

	started, msg, err := r.qm.AddToQueue(repo, pr)
	if err != nil {
		return fmt.Errorf("failed to add to queue: %s: %w", pr.GetName(), err)
	}

	if started {
		if err := r.updatePipelineRunToInProgress(ctx, logger, repo, pr); err != nil {
			return fmt.Errorf("failed to update pipelineRun to in_progress: %w", err)
		}
		return nil
	}

	logger.Infof("pipelineRun %s yet to start, %s", pr.Name, msg)
	return nil
}

func (r *Reconciler) reportFinalStatus(ctx context.Context, logger *zap.SugaredLogger, event *info.Event, pr *v1beta1.PipelineRun, provider provider.Interface) error {
	repoName := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "repository")]
	repo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().
		Repositories(pr.Namespace).Get(ctx, repoName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("reportFinalStatus: %w", err)
	}

	if repo.Spec.GitProvider != nil {
		if err := pipelineascode.SecretFromRepository(ctx, r.run, r.kinteract, provider.GetConfig(), event, repo, logger); err != nil {
			return fmt.Errorf("cannot get secret from repository: %w", err)
		}
	} else {
		event.Provider.WebhookSecret, _ = pipelineascode.GetCurrentNSWebhookSecret(ctx, r.kinteract)
	}

	err = provider.SetClient(ctx, event)
	if err != nil {
		return fmt.Errorf("cannot set client: %w", err)
	}

	if err := r.cleanupPipelineRuns(ctx, logger, repo, pr); err != nil {
		return fmt.Errorf("cannot clean prs: %w", err)
	}

	if r.run.Info.Pac.SecretAutoCreation {
		if err := r.cleanupSecrets(ctx, logger, repo, pr); err != nil {
			return fmt.Errorf("cannot clean secret: %w", err)
		}
	}

	newPr, err := r.postFinalStatus(ctx, logger, provider, event, pr)
	if err != nil {
		return fmt.Errorf("cannot post final status: %w", err)
	}

	if err := r.updateRepoRunStatus(ctx, logger, newPr, repo, event); err != nil {
		return fmt.Errorf("cannot update run status: %w", err)
	}

	if _, err := r.updatePipelineRunState(ctx, logger, pr, kubeinteraction.StateCompleted); err != nil {
		return fmt.Errorf("cannot update state: %w", err)
	}

	// remove pipelineRun from Queue and start the next one
	next := r.qm.RemoveFromQueue(repo, pr)
	if next != "" {
		key := strings.Split(next, "/")
		pr, err := r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(key[0]).Get(ctx, key[1], metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("cannot get pipeline: %w", err)
		}
		if err := r.updatePipelineRunToInProgress(ctx, logger, repo, pr); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
		return nil
	}

	return nil
}

func (r *Reconciler) updatePipelineRunToInProgress(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	pr, err := r.updatePipelineRunState(ctx, logger, pr, kubeinteraction.StateStarted)
	if err != nil {
		return fmt.Errorf("cannot update state: %w", err)
	}

	p, event, err := r.detectProvider(ctx, logger, pr)
	if err != nil {
		logger.Error(err)
		return nil
	}

	if repo.Spec.GitProvider != nil {
		if err := pipelineascode.SecretFromRepository(ctx, r.run, r.kinteract, p.GetConfig(), event, repo, logger); err != nil {
			return fmt.Errorf("cannot get secret from repo: %w", err)
		}
	} else {
		event.Provider.WebhookSecret, _ = pipelineascode.GetCurrentNSWebhookSecret(ctx, r.kinteract)
	}

	err = p.SetClient(ctx, event)
	if err != nil {
		return fmt.Errorf("cannot set client: %w", err)
	}

	consoleURL := r.run.Clients.ConsoleUI.DetailURL(repo.GetNamespace(), pr.GetName())
	// Create status with the log url
	msg := fmt.Sprintf(startingPipelineRunText, pr.GetName(), repo.GetNamespace(), consoleURL,
		repo.GetNamespace(), pr.GetName())
	status := provider.StatusOpts{
		Status:                  "in_progress",
		Conclusion:              "pending",
		Text:                    msg,
		DetailsURL:              consoleURL,
		PipelineRunName:         pr.GetName(),
		PipelineRun:             pr,
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "original-prname")],
	}

	if err := p.CreateStatus(ctx, r.run.Clients.Tekton, event, r.run.Info.Pac, status); err != nil {
		return fmt.Errorf("cannot create a in_progress status on the provider platform: %w", err)
	}

	logger.Info("updated in_progress status on provider platform for pipelineRun ", pr.GetName())
	return nil
}

func (r *Reconciler) updatePipelineRunState(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun, state string) (*v1beta1.PipelineRun, error) {
	maxRun := 10
	for i := 0; i < maxRun; i++ {
		newPr, err := r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(pr.Namespace).Get(ctx, pr.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("cannot get pipeline: %w", err)
		}

		newPr = newPr.DeepCopy()
		newPr.Labels[filepath.Join(pipelinesascode.GroupName, "state")] = state

		if state == kubeinteraction.StateStarted {
			newPr.Spec.Status = ""
		}

		updatedPR, err := r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(newPr.Namespace).Update(ctx, newPr, metav1.UpdateOptions{})
		if err != nil {
			logger.Infof("could not update Pipelinerun with State change, retrying %v/%v: %v", newPr.GetNamespace(), newPr.GetName(), err)
			continue
		}
		logger.Infof("updated pac state in pipelinerun: %v/%v", updatedPR.Namespace, updatedPR.Name)
		return updatedPR, nil
	}
	return nil, nil
}
