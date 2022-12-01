package kubeinteraction

import (
	"context"

	ktypes "github.com/openshift-pipelines/pipelines-as-code/pkg/secrets/types"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k Interaction) GetSecret(ctx context.Context, secretopt ktypes.GetSecretOpt) (string, error) {
	secret, err := k.Run.Clients.Kube.CoreV1().Secrets(secretopt.Namespace).Get(
		ctx, secretopt.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[secretopt.Key]), nil
}

// DeleteSecret deletes the secret created for git-clone basic-auth
func (k Interaction) DeleteSecret(ctx context.Context, logger *zap.SugaredLogger, targetNamespace, secretName string) error {
	err := k.Run.Clients.Kube.CoreV1().Secrets(targetNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	logger.Infof("Secret %s has been deleted in namespace %s", secretName, targetNamespace)
	return nil
}

func (k Interaction) CreateSecret(ctx context.Context, ns string, secret *corev1.Secret) error {
	_, err := k.Run.Clients.Kube.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	return err
}
