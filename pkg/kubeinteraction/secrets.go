package kubeinteraction

import (
	"context"
	"fmt"
	"net/url"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
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
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: map[string]string{"app.kubernetes.io/managed-by": "pipelines-as-code"},
		},
	}
	secret.StringData = secretData
	_, err := k.Run.Clients.Kube.CoreV1().Secrets(targetNamespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

const defaultGitUser = "git"

// CreateBasicAuthSecret Create a secret for git-clone basic-auth workspace
func (k Interaction) CreateBasicAuthSecret(ctx context.Context, runevent *info.Event, pacopts info.PacOpts, targetNamespace string) error {
	repoURL, err := url.Parse(runevent.URL)
	if err != nil {
		return err
	}

	gitUser := defaultGitUser
	if pacopts.VCSUser != "" {
		gitUser = pacopts.VCSUser
	}

	urlWithToken := fmt.Sprintf("%s://%s:%s@%s%s", repoURL.Scheme, gitUser, pacopts.VCSToken, repoURL.Host, repoURL.Path)
	secretData := map[string]string{
		".gitconfig":       fmt.Sprintf(basicAuthGitConfigData, runevent.URL),
		".git-credentials": urlWithToken,
	}

	// Try to create secrete if that fails then delete it first and then create
	// This allows up not to give List and Get right clusterwide
	secretName := fmt.Sprintf(basicAuthSecretName, runevent.Owner, runevent.Repository)
	err = k.createSecret(ctx, secretData, targetNamespace, secretName)
	if err != nil {
		err = k.Run.Clients.Kube.CoreV1().Secrets(targetNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err == nil {
			err = k.createSecret(ctx, secretData, targetNamespace, secretName)
		}
	}
	k.Run.Clients.Log.Infof("Secret %s has been generated in namespace %s", secretName, targetNamespace)
	return err
}

func (k Interaction) GetSecret(ctx context.Context, secretopt GetSecretOpt) (string, error) {
	secret, err := k.Run.Clients.Kube.CoreV1().Secrets(secretopt.Namespace).Get(
		ctx, secretopt.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[secretopt.Key]), nil
}
