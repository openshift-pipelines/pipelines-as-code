package webhook

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	providerTokenKey = "provider.token"
	webhookSecretKey = "webhook.secret"
)

func (w Webhook) createWebhookSecret(ctx context.Context, kubeClient kubernetes.Interface, secretName string, response *response) error {
	_, err := kubeClient.CoreV1().Secrets(w.RepositoryNamespace).Create(ctx, &corev1.Secret{
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

func (w Webhook) updateRepositoryCR(ctx context.Context, pacClient versioned.Interface, secretName string) error {
	repo, err := pacClient.PipelinesascodeV1alpha1().Repositories(w.RepositoryNamespace).
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

	_, err = pacClient.PipelinesascodeV1alpha1().Repositories(w.RepositoryNamespace).
		Update(ctx, repo, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🔑 Repository CR %s has been updated with webhook secret in the %s namespace\n", w.RepositoryName, w.RepositoryNamespace)
	return nil
}
