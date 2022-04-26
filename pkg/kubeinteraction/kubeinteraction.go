package kubeinteraction

import (
	"context"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"

	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
)

type GetSecretOpt struct {
	Namespace string
	Name      string
	Key       string
}

type Interface interface {
	WaitForPipelineRunSucceed(context.Context, tektonv1beta1client.TektonV1beta1Interface, *v1beta1.PipelineRun, time.Duration) error
	CleanupPipelines(context.Context, *zap.SugaredLogger, *v1alpha1.Repository, *v1beta1.PipelineRun, int) error
	CreateBasicAuthSecret(context.Context, *zap.SugaredLogger, *info.Event, string) (string, error)
	DeleteBasicAuthSecret(context.Context, *zap.SugaredLogger, string, string) error
	GetSecret(context.Context, GetSecretOpt) (string, error)
}

type Interaction struct {
	Run *params.Run
}

func NewKubernetesInteraction(c *params.Run) (*Interaction, error) {
	return &Interaction{
		Run: c,
	}, nil
}
