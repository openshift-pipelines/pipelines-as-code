package kubernetestint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
)

type KinterfaceTest struct {
	ConsoleURL               string
	NamespaceError           bool
	ExpectedNumberofCleanups int
}

func (k *KinterfaceTest) GetConsoleUI(ctx context.Context, ns string, pr string) (string, error) {
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) GetNamespace(ctx context.Context, ns string) error {
	if k.NamespaceError {
		return errors.New("cannot find namespace")
	}
	return nil
}

func (k *KinterfaceTest) WaitForPipelineRunSucceed(ctx context.Context, tektonbeta1 tektonv1beta1client.TektonV1beta1Interface, pr *v1beta1.PipelineRun, polltimeout time.Duration) error {
	return nil
}

func (k *KinterfaceTest) CleanupPipelines(ctx context.Context, namespace string, runinfo *webvcs.RunInfo, maxKeep int) error {
	if k.ExpectedNumberofCleanups != maxKeep {
		return fmt.Errorf("we wanted %d and we got %d", k.ExpectedNumberofCleanups, maxKeep)
	}
	return nil
}
