package kubernetestint

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	ktypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type KinterfaceTest struct {
	ConsoleURL               string
	ConsoleURLErorring       bool
	ExpectedNumberofCleanups int
	GetSecretResult          map[string]string
	GetPodLogsOutput         map[string]string
}

var _ kubeinteraction.Interface = (*KinterfaceTest)(nil)

func (k *KinterfaceTest) GetConsoleUI(_ context.Context, _, _ string) (string, error) {
	if k.ConsoleURLErorring {
		return "", fmt.Errorf("i want you to errit")
	}
	return k.ConsoleURL, nil
}

func (k *KinterfaceTest) GetPodLogs(_ context.Context, _, pod, _ string, _ int64) (string, error) {
	if ok := k.GetPodLogsOutput[pod]; ok != "" {
		return k.GetPodLogsOutput[pod], nil
	}
	return "", nil
}

func (k *KinterfaceTest) UpdateSecretWithOwnerRef(_ context.Context, _ *zap.SugaredLogger, _, _ string, _ *tektonv1.PipelineRun) error {
	return nil
}

func (k *KinterfaceTest) GetSecret(_ context.Context, secret ktypes.GetSecretOpt) (string, error) {
	if _, ok := k.GetSecretResult[secret.Name]; !ok {
		return "", fmt.Errorf("secret %s does not exist", secret.Name)
	}
	return k.GetSecretResult[secret.Name], nil
}

func (k *KinterfaceTest) CleanupPipelines(_ context.Context, _ *zap.SugaredLogger, _ *v1alpha1.Repository,
	_ *tektonv1.PipelineRun, limitnumber int,
) error {
	if k.ExpectedNumberofCleanups != limitnumber {
		return fmt.Errorf("we wanted %d and we got %d", k.ExpectedNumberofCleanups, limitnumber)
	}
	return nil
}

func (k *KinterfaceTest) CreateSecret(_ context.Context, _ string, _ *corev1.Secret) error {
	return nil
}

func (k *KinterfaceTest) DeleteSecret(_ context.Context, _ *zap.SugaredLogger, _, _ string) error {
	return nil
}
