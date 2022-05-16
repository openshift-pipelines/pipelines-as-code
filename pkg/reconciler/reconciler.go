package reconciler

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1beta12 "github.com/tektoncd/pipeline/pkg/client/informers/externalversions/pipeline/v1beta1"
	pipelinerunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/pipelinerun"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	run         *params.Run
	pipelinerun v1beta12.PipelineRunInformer
	kinteract   *kubeinteraction.Interaction
}

var (
	_ pipelinerunreconciler.Interface = (*Reconciler)(nil)
)

func (c *Reconciler) ReconcileKind(ctx context.Context, pr *v1beta1.PipelineRun) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	logger.Info("Checking status for pipelineRun: ", pr.GetName())

	if pr.IsDone() {
		logger.Infof("pipelineRun %v is done !!!  ", pr.GetName())
		return nil

	}

	logger.Infof("pipelineRun %v is not done ...  ", pr.GetName())
	return nil
}
