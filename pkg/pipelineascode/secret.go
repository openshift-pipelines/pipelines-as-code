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
func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.ProviderConfig, repo *apipac.Repository) error {
	var err error
	if repo.Spec.GitProvider.URL == "" {
		repo.Spec.GitProvider.URL = config.APIURL
	} else {
		cs.Info.Pac.ProviderURL = repo.Spec.GitProvider.URL
	}

	key := repo.Spec.GitProvider.Secret.Key
	if key == "" {
		key = defaultGitProviderSecretKey
	}

	cs.Info.Pac.ProviderUser = repo.Spec.GitProvider.User
	cs.Info.Pac.ProviderToken, err = k8int.GetSecret(
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
	cs.Info.Pac.ProviderInfoFromRepo = true

	cs.Clients.Log.Infof("Using git provider %s: apiurl=%s user=%s token-secret=%s in token-key=%s",
		cs.Info.Pac.WebhookType,
		repo.Spec.GitProvider.URL,
		repo.Spec.GitProvider.User,
		repo.Spec.GitProvider.Secret.Name,
		key)

	return nil
}
