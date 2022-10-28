package kubernetestint

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
)

type KinterfaceTest struct {
	ConsoleURL               string
	ConsoleURLErorring       bool
	ExpectedNumberofCleanups int
	GetSecretResult          map[string]string
}

var _ kubeinteraction.Interface = (*KinterfaceTest)(nil)

func (k *KinterfaceTest) GetConsoleUI(_ context.Context, _, _ string) (string, error) {
	if k.ConsoleURLErorring {
		return "", fmt.Errorf("i want you to errit")
	}
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) CreateBasicAuthSecret(context.Context, *zap.SugaredLogger, *info.Event, string, string) error {
	return nil
}

func (k *KinterfaceTest) DeleteBasicAuthSecret(_ context.Context, _ *zap.SugaredLogger, _, _ string) error {
	return nil
}

func (k *KinterfaceTest) GetSecret(_ context.Context, secretopt kubeinteraction.GetSecretOpt) (string, error) {
	// check if secret exist in k.GetSecretResult
	if k.GetSecretResult[secretopt.Name] == "" {
		return "", fmt.Errorf("secret %s does not exist", secretopt.Name)
	}
	return k.GetSecretResult[secretopt.Name], nil
}

func (k *KinterfaceTest) CleanupPipelines(_ context.Context, _ *zap.SugaredLogger, _ *v1alpha1.Repository,
	pr *v1beta1.PipelineRun, limitnumber int,
) error {
	if k.ExpectedNumberofCleanups != limitnumber {
		return fmt.Errorf("we wanted %d and we got %d", k.ExpectedNumberofCleanups, limitnumber)
	}
	return nil
}
