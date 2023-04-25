package reconciler

import (
	"context"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

func (r *Reconciler) FinalizeKind(ctx context.Context, pr *tektonv1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	state, exist := pr.GetAnnotations()[keys.State]
	if !exist || state == kubeinteraction.StateCompleted {
		return nil
	}

	if state == kubeinteraction.StateQueued || state == kubeinteraction.StateStarted {
		repoName, ok := pr.GetAnnotations()[keys.Repository]
		if !ok {
			return nil
		}
		repo, err := r.repoLister.Repositories(pr.Namespace).Get(repoName)
		// if repository is not found then remove the queue for that repository if exist
		if errors.IsNotFound(err) {
			r.qm.RemoveRepository(&v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{Name: repoName, Namespace: pr.Namespace},
			})
			return nil
		}
		if err != nil {
			return err
		}

		logger = logger.With("namespace", repo.Namespace)
		next := r.qm.RemoveFromQueue(repo, pr)
		if next != "" {
			key := strings.Split(next, "/")
			pr, err := r.run.Clients.Tekton.TektonV1().PipelineRuns(key[0]).Get(ctx, key[1], metav1.GetOptions{})
			if err != nil {
				return err
			}
			if err := r.updatePipelineRunToInProgress(ctx, logger, repo, pr); err != nil {
				logger.Error("failed to update status: ", err)
				return err
			}
			return nil
		}
	}
	return nil
}
