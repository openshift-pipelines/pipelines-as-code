package kubeinteraction

import (
	"context"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
)

type GetSecretOpt struct {
	Namespace string
	Name      string
	Key       string
}

type Interface interface {
	GetConsoleUI(context.Context, string, string) (string, error)
	WaitForPipelineRunSucceed(context.Context, tektonv1beta1client.TektonV1beta1Interface, *v1beta1.PipelineRun, time.Duration) error
	CleanupPipelines(context.Context, string, string, int) error
	CreateBasicAuthSecret(context.Context, *info.Event, *info.PacOpts, string) error
	GetSecret(context.Context, GetSecretOpt) (string, error)
}

type Interaction struct {
	Run *params.Run
}

func (k Interaction) GetConsoleUI(ctx context.Context, ns, pr string) (string, error) {
	return consoleui.GetConsoleUI(ctx, k.Run, ns, pr)
}

func NewKubernetesInteraction(c *params.Run) (*Interaction, error) {
	return &Interaction{
		Run: c,
	}, nil
}
