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

	secretName := fmt.Sprintf(basicAuthSecretName, runinfo.Owner, runinfo.Repository)
	secret, err := k.Clients.Kube.CoreV1().Secrets(targetNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName}}
		secret.StringData = secretData
		_, err = k.Clients.Kube.CoreV1().Secrets(targetNamespace).Create(ctx, secret, metav1.CreateOptions{})
	} else {
		secret.StringData = secretData
		_, err = k.Clients.Kube.CoreV1().Secrets(targetNamespace).Update(ctx, secret, metav1.UpdateOptions{})
	}

	return err
}
