package reconciler

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	run               *params.Run
	pipelineRunLister v1beta12.PipelineRunLister
	kinteract         kubeinteraction.Interface
	// added for testing
	provider provider.Interface
}

var _ pipelinerunreconciler.Interface = (*Reconciler)(nil)

var gitAuthSecretAnnotation = "pipelinesascode.tekton.dev/git-auth-secret"

func (r *Reconciler) ReconcileKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	if pr.IsDone() {
		logger = logger.With(
			"pipeline-run", pr.GetName(),
			"event-sha", pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "sha")],
		)
		logger.Infof("pipelineRun %v/%v is done, reconciling to report status!  ", pr.GetNamespace(), pr.GetName())

		return r.reportStatus(ctx, logger, pr)
	}
	return nil
}

func (r *Reconciler) reportStatus(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun) error {
	prLabels := pr.GetLabels()

	// fetch repository CR for pipelineRun
	repoName := prLabels[filepath.Join(pipelinesascode.GroupName, "repository")]
	repo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().
		Repositories(pr.Namespace).Get(ctx, repoName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := r.cleanupPipelineRuns(ctx, logger, repo, pr); err != nil {
		return err
	}

	provider, event, err := r.detectProvider(ctx, pr)
	if err != nil {
		logger.Error(err)
		return nil
	}
	provider.SetLogger(logger)

	if repo.Spec.GitProvider != nil {
		if err := pipelineascode.SecretFromRepository(ctx, r.run, r.kinteract, provider.GetConfig(), event, repo, logger); err != nil {
			return err
		}
	} else {
		event.Provider.WebhookSecret, _ = pipelineascode.GetCurrentNSWebhookSecret(ctx, r.kinteract)
	}

	if r.run.Info.Pac.SecretAutoCreation {
		if err := r.cleanupSecrets(ctx, logger, repo, pr); err != nil {
			return err
		}
	}

	err = provider.SetClient(ctx, event)
	if err != nil {
		return err
	}

	newPr, err := r.postFinalStatus(ctx, logger, provider, event, pr)
	if err != nil {
		return err
	}

	if err := r.updateRepoRunStatus(ctx, logger, newPr, repo, event); err != nil {
		return err
	}

	return r.updatePipelineRunState(ctx, logger, pr)
}

func (r *Reconciler) cleanupSecrets(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	var gitAuthSecretName string
	if annotation, ok := pr.Annotations[gitAuthSecretAnnotation]; ok {
		gitAuthSecretName = annotation
	} else {
		return fmt.Errorf("cannot get annotation %s as set on PR", gitAuthSecretAnnotation)
	}

	err := r.kinteract.DeleteBasicAuthSecret(ctx, logger, repo.GetNamespace(), gitAuthSecretName)
	if err != nil {
		return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
	}
	return nil
}

// Cleanup old succeeded pipelineRuns
func (r *Reconciler) cleanupPipelineRuns(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun) error {
	keepMaxPipeline, ok := pr.Annotations[filepath.Join(pipelinesascode.GroupName, "max-keep-runs")]
	if ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}

		err = r.kinteract.CleanupPipelines(ctx, logger, repo, pr, max)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updatePipelineRunState(ctx context.Context, logger *zap.SugaredLogger, pr *v1beta1.PipelineRun) error {
	newPr, err := r.pipelineRunLister.PipelineRuns(pr.Namespace).Get(pr.Name)
	if err != nil {
		return fmt.Errorf("error getting PipelineRun %s when updating state: %w", pr.Name, err)
	}

	newPr = newPr.DeepCopy()
	newPr.Labels[filepath.Join(pipelinesascode.GroupName, "state")] = kubeinteraction.StateCompleted

	_, err = r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(pr.Namespace).Update(ctx, newPr, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	logger.Infof("updated pac state in pipelinerun")
	return err
}
