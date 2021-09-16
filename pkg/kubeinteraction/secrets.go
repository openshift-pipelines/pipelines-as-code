package kubeinteraction

import (
	"context"
	"fmt"
	"net/url"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// nolint:gosec
	basicAuthSecretName    = `pac-git-basic-auth-%s-%s`
	basicAuthGitConfigData = `
	[credential "%s"]
	helper=store
	`
)

func (k Interaction) createSecret(ctx context.Context, secretData map[string]string, targetNamespace, secretName string) error {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:   secretName,
		Labels: map[string]string{"app.kubernetes.io/managed-by": "pipelines-as-code"},
	}}
	secret.StringData = secretData
	_, err := k.Clients.Kube.CoreV1().Secrets(targetNamespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// CreateBasicAuthSecret Create a secret for git-clone basic-auth workspace
func (k Interaction) CreateBasicAuthSecret(ctx context.Context, runinfo webvcs.RunInfo, targetNamespace, token string) error {
	repoURL, err := url.Parse(runinfo.URL)
	if err != nil {
		return err
	}
	urlWithToken := fmt.Sprintf("%s://git:%s@%s%s", repoURL.Scheme, token, repoURL.Host, repoURL.Path)

	secretData := map[string]string{
		".gitconfig":       fmt.Sprintf(basicAuthGitConfigData, runinfo.URL),
		".git-credentials": urlWithToken,
	}

	// Try to create secrete if that fails then delete it first and then create
	// This allows up not to give List and Get right clusterwide
	secretName := fmt.Sprintf(basicAuthSecretName, runinfo.Owner, runinfo.Repository)
	err = k.createSecret(ctx, secretData, targetNamespace, secretName)
	if err != nil {
		err = k.Clients.Kube.CoreV1().Secrets(targetNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err == nil {
			err = k.createSecret(ctx, secretData, targetNamespace, secretName)
		}
	}

	return err
}
