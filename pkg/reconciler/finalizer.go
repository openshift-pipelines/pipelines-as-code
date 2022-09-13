package reconciler

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

func (r *Reconciler) FinalizeKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	state, exist := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "state")]
	if !exist || state == kubeinteraction.StateCompleted {
		return nil
	}

	if state == kubeinteraction.StateQueued || state == kubeinteraction.StateStarted {
		repoName, ok := pr.GetLabels()[filepath.Join(pipelinesascode.GroupName, "repository")]
		if !ok {
			return nil
		}
		repo, err := r.run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().
			Repositories(pr.Namespace).Get(ctx, repoName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		next := r.qm.RemoveFromQueue(repo, pr)
		if next != "" {
			key := strings.Split(next, "/")
			pr, err := r.run.Clients.Tekton.TektonV1beta1().PipelineRuns(key[0]).Get(ctx, key[1], metav1.GetOptions{})
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
