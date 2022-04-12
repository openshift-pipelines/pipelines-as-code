package pipelineascode

import (
	"context"
	"fmt"
	"os"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

const (
	defaultGitProviderSecretKey                  = "provider.token"
	defaultGitProviderWebhookSecretKey           = "webhook.secret"
	defaultPipelinesAscodeSecretName             = "pipelines-as-code-secret"
	defaultPipelinesAscodeSecretWebhookSecretKey = "webhook.secret"
)

// secretFromRepository grab the secret from the repository CRD
func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.ProviderConfig, event *info.Event, repo *apipac.Repository) error {
	var err error
	if repo.Spec.GitProvider.URL == "" {
		repo.Spec.GitProvider.URL = config.APIURL
	} else {
		event.Provider.URL = repo.Spec.GitProvider.URL
	}

	gitProviderSecretKey := repo.Spec.GitProvider.Secret.Key
	if gitProviderSecretKey == "" {
		gitProviderSecretKey = defaultGitProviderSecretKey
	}
	if event.Provider.Token, err = k8int.GetSecret(ctx, kubeinteraction.GetSecretOpt{
		Namespace: repo.GetNamespace(),
		Name:      repo.Spec.GitProvider.Secret.Name,
		Key:       gitProviderSecretKey,
	}); err != nil {
		return err
	}
	// if we don't have a provider token in repo crd we won't be able to do much with it
	// let it go and it will fail later on when doing SetClients or success if it was done from a github app
	if event.Provider.Token == "" {
		return nil
	}
	event.Provider.InfoFromRepo = true
	event.Provider.User = repo.Spec.GitProvider.User

	if repo.Spec.GitProvider.WebhookSecret == nil {
		// repo.Spec.GitProvider.url/token without a webhook secret is probably going to be bitbucket cloud which
		// doesn't have webhook support 🙃
		return nil
	}

	gitProviderWebhookSecretKey := repo.Spec.GitProvider.WebhookSecret.Key
	if gitProviderWebhookSecretKey == "" {
		gitProviderWebhookSecretKey = defaultGitProviderWebhookSecretKey
	}
	logmsg := fmt.Sprintf("Using git provider %s: apiurl=%s user=%s token-secret=%s token-key=%s",
		cs.Info.Pac.WebhookType,
		repo.Spec.GitProvider.URL,
		repo.Spec.GitProvider.User,
		repo.Spec.GitProvider.Secret.Name,
		gitProviderSecretKey)
	if event.Provider.WebhookSecret, err = k8int.GetSecret(ctx, kubeinteraction.GetSecretOpt{
		Namespace: repo.GetNamespace(),
		Name:      repo.Spec.GitProvider.WebhookSecret.Name,
		Key:       gitProviderWebhookSecretKey,
	}); err != nil {
		return err
	}
	if event.Provider.WebhookSecret != "" {
		event.Provider.WebhookSecretFromRepo = true
		logmsg += fmt.Sprintf(" webhook-secret=%s webhook-key=%s",
			repo.Spec.GitProvider.WebhookSecret.Name,
			gitProviderWebhookSecretKey)
	}
	cs.Clients.Log.Infof(logmsg)
	return nil
}

// getCurrentNSWebhookSecret get secret from current namespace if it exists
func getCurrentNSWebhookSecret(ctx context.Context, k8int kubeinteraction.Interface) (string, error) {
	s, err := k8int.GetSecret(ctx, kubeinteraction.GetSecretOpt{
		Namespace: os.Getenv("SYSTEM_NAMESPACE"),
		Name:      defaultPipelinesAscodeSecretName,
		Key:       defaultPipelinesAscodeSecretWebhookSecretKey,
	})
	// a lot of people have problem with this secret, when encoding it to base64 which add a \n when we do :
	// echo secret|base64 -w0
	// so cleanup, if someone wants to have a \n or a space in the secret, well then they can't :p
	return strings.TrimSpace(s), err
}
