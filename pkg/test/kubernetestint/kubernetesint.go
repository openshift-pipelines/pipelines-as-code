package kubernetestint

import (
	"context"
	"errors"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
)

type KinterfaceTest struct {
	ConsoleURL     string
	NamespaceError bool
}

func (k *KinterfaceTest) GetConsoleUI(ctx context.Context, ns string, pr string) (string, error) {
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) GetNamespace(ctx context.Context, ns string) error {
	if k.NamespaceError {
		return errors.New("cannot find Namespace")
	}
	return nil
}

func (k *KinterfaceTest) WaitForPipelineRunSucceed(ctx context.Context, tektonbeta1 tektonv1beta1client.TektonV1beta1Interface, pr *v1beta1.PipelineRun, polltimeout time.Duration) error {
	return nil
}
