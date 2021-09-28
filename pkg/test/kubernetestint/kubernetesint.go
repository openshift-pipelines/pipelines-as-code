package kubernetestint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
)

type KinterfaceTest struct {
	ConsoleURL               string
	ConsoleURLErorring       bool
	NamespaceError           bool
	ExpectedNumberofCleanups int
	GetSecretResult          string
}

func (k *KinterfaceTest) GetConsoleUI(ctx context.Context, ns string, pr string) (string, error) {
	if k.ConsoleURLErorring {
		return "", fmt.Errorf("i want you to errit")
	}
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) CreateBasicAuthSecret(ctx context.Context, runevent *info.Event, pacopts info.PacOpts, targetNamespace string) error {
	return nil
}

func (k *KinterfaceTest) GetSecret(ctx context.Context, secretopt kubeinteraction.GetSecretOpt) (string, error) {
	return k.GetSecretResult, nil
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

func (k *KinterfaceTest) CleanupPipelines(ctx context.Context, namespace string, repoName string, maxKeep int) error {
	if k.ExpectedNumberofCleanups != maxKeep {
		return fmt.Errorf("we wanted %d and we got %d", k.ExpectedNumberofCleanups, maxKeep)
	}
	return nil
}
