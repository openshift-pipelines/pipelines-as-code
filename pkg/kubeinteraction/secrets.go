package kubeinteraction

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
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

// CreateBasicAuthSecret Create a secret for git-clone basic-auth workspace
func (k Interaction) CreateBasicAuthSecret(ctx context.Context, runevent *info.Event, pacopts *info.PacOpts, targetNamespace string) error {
	// Bitbucket Server have a different Clone URL than it's Repo URL, so we
	// have to separate them 👨‍🏭
	cloneURL := runevent.URL
	if runevent.CloneURL != "" {
		cloneURL = runevent.CloneURL
	}

	repoURL, err := url.Parse(cloneURL)
	if err != nil {
		return fmt.Errorf("cannot parse url %s: %w", cloneURL, err)
	}

	gitUser := webvcs.DefaultWebvcsAPIUser
	if pacopts.VCSUser != "" {
		gitUser = pacopts.VCSUser
	}

	// Bitbucket server token have / into it, so unless we do urlquote them it's
	// impossible to use it🤡
	//
	// It supposed not working on github according to
	// https://stackoverflow.com/a/24719496 but arguably github have a better
	// product and would not do such things.
	//
	// maybe we could patch the git-clone task too but that probably be a pain
	// in the tuchus to do it in shell.
	token := url.QueryEscape(pacopts.VCSToken)

	urlWithToken := fmt.Sprintf("%s://%s:%s@%s%s", repoURL.Scheme, gitUser, token, repoURL.Host, repoURL.Path)
	secretData := map[string]string{
		".gitconfig":       fmt.Sprintf(basicAuthGitConfigData, cloneURL),
		".git-credentials": urlWithToken,
	}

	// Try to create secrete if that fails then delete it first and then create
	// This allows up not to give List and Get right clusterwide
	secretName := fmt.Sprintf(basicAuthSecretName, strings.ToLower(runevent.Owner), strings.ToLower(runevent.Repository))
	err = k.createSecret(ctx, secretData, targetNamespace, secretName)
	if err != nil {
		err = k.Run.Clients.Kube.CoreV1().Secrets(targetNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err == nil {
			err = k.createSecret(ctx, secretData, targetNamespace, secretName)
		}
	}

	if err != nil {
		return fmt.Errorf("cannot create secret: %w", err)
	}

	k.Run.Clients.Log.Infof("Secret %s has been generated in namespace %s", secretName, targetNamespace)
	return nil
}

func (k Interaction) GetSecret(ctx context.Context, secretopt GetSecretOpt) (string, error) {
	secret, err := k.Run.Clients.Kube.CoreV1().Secrets(secretopt.Namespace).Get(
		ctx, secretopt.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data[secretopt.Key]), nil
}
