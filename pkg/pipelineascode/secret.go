package pipelineascode

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

const (
	defaultGitProviderSecretKey = "token"
)

// secretFromRepository grab the secret from the repository CRD
func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.ProviderConfig, event *info.Event, repo *apipac.Repository) error {
	var err error
	if repo.Spec.GitProvider.URL == "" {
		repo.Spec.GitProvider.URL = config.APIURL
	} else {
		event.ProviderURL = repo.Spec.GitProvider.URL
	}

	key := repo.Spec.GitProvider.Secret.Key
	if key == "" {
		key = defaultGitProviderSecretKey
	}

	event.ProviderUser = repo.Spec.GitProvider.User
	event.ProviderToken, err = k8int.GetSecret(
		ctx,
		kubeinteraction.GetSecretOpt{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.GitProvider.Secret.Name,
			Key:       key,
		},
	)

	if err != nil {
		return err
	}
	event.ProviderInfoFromRepo = true

	cs.Clients.Log.Infof("Using git provider %s: apiurl=%s user=%s token-secret=%s in token-key=%s",
		cs.Info.Pac.WebhookType,
		repo.Spec.GitProvider.URL,
		repo.Spec.GitProvider.User,
		repo.Spec.GitProvider.Secret.Name,
		key)

	return nil
}
