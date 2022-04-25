package webhook

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	providerTokenKey = "provider.token"
	webhookSecretKey = "webhook.secret"

	// nolint
	githubWebhookSecretName = "github-webhook-secret"
	// nolint
	gitlabWebhookSecretName = "gitlab-webhook-secret"
)

func (w *Options) createWebhookSecret(ctx context.Context, provider string, response *response) error {
	secretName := getSecretName(provider)
	_, err := w.Run.Clients.Kube.CoreV1().Secrets(w.RepositoryNamespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			providerTokenKey: []byte(response.PersonalAccessToken),
			webhookSecretKey: []byte(response.WebhookSecret),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🔑 Webhook Secret %s has been created in the %s namespace\n", secretName, w.RepositoryNamespace)
	return nil
}

func (w *Options) updateRepositoryCR(ctx context.Context, provider string) error {
	secretName := getSecretName(provider)
	repo, err := w.Run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(w.RepositoryNamespace).
		Get(ctx, w.RepositoryName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if repo.Spec.GitProvider == nil {
		repo.Spec.GitProvider = &v1alpha1.GitProvider{}
	}

	repo.Spec.GitProvider.Secret = &v1alpha1.GitProviderSecret{
		Name: secretName,
		Key:  providerTokenKey,
	}
	repo.Spec.GitProvider.WebhookSecret = &v1alpha1.GitProviderSecret{
		Name: secretName,
		Key:  webhookSecretKey,
	}

	_, err = w.Run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(w.RepositoryNamespace).
		Update(ctx, repo, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🔑 Repository CR %s has been updated with webhook secret in the %s namespace\n", w.RepositoryName, w.RepositoryNamespace)
	return nil
}

func getSecretName(providerName string) string {
	switch providerName {
	case provider.ProviderGitHubWebhook:
		return githubWebhookSecretName
	case provider.ProviderGitLabWebhook:
		return gitlabWebhookSecretName
	default:
		return ""
	}
}
