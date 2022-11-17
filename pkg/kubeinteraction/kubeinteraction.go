package kubeinteraction

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

type GetSecretOpt struct {
	Namespace string
	Name      string
	Key       string
}

type Interface interface {
	CleanupPipelines(context.Context, *zap.SugaredLogger, *v1alpha1.Repository, *v1beta1.PipelineRun, int) error
	CreateBasicAuthSecret(context.Context, *zap.SugaredLogger, *info.Event, string, string) error
	DeleteBasicAuthSecret(context.Context, *zap.SugaredLogger, string, string) error
	GetSecret(context.Context, GetSecretOpt) (string, error)
	GetPodLogs(context.Context, string, string, string, int64) (string, error)
}

type Interaction struct {
	Run *params.Run
}

// validate the interface implementation
var _ Interface = (*Interaction)(nil)

func NewKubernetesInteraction(c *params.Run) (*Interaction, error) {
	return &Interaction{
		Run: c,
	}, nil
}
