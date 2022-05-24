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

func (k *KinterfaceTest) GetConsoleUI(ctx context.Context, ns string, pr string) (string, error) {
	if k.ConsoleURLErorring {
		return "", fmt.Errorf("i want you to errit")
	}
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) CreateBasicAuthSecret(ctx context.Context, logger *zap.SugaredLogger, runevent *info.Event,
	targetNamespace, secretName string,
) error {
	return nil
}

func (k *KinterfaceTest) DeleteBasicAuthSecret(ctx context.Context, logger *zap.SugaredLogger, targetNamespace, secretName string) error {
	return nil
}

func (k *KinterfaceTest) GetSecret(_ context.Context, secretopt kubeinteraction.GetSecretOpt) (string, error) {
	return k.GetSecretResult[secretopt.Name], nil
}

func (k *KinterfaceTest) CleanupPipelines(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun, limitnumber int) error {
	if k.ExpectedNumberofCleanups != limitnumber {
		return fmt.Errorf("we wanted %d and we got %d", k.ExpectedNumberofCleanups, limitnumber)
	}
	return nil
}
