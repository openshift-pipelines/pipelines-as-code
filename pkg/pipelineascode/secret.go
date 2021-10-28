package pipelineascode

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

const (
	defaultWebvcsAPISecretKey = "token"
)

// secretFromRepository grab the secret from the repository CRD
func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.VCSConfig, repo *apipac.Repository) error {
	var err error

	if repo.Spec.WebvcsAPIURL == "" {
		repo.Spec.WebvcsAPIURL = config.APIURL
	} else {
		cs.Info.Pac.VCSAPIURL = repo.Spec.WebvcsAPIURL
	}

	key := repo.Spec.WebvcsAPISecret.Key
	if key == "" {
		key = defaultWebvcsAPISecretKey
	}

	cs.Info.Pac.VCSUser = repo.Spec.WebvcsAPIUser
	cs.Info.Pac.VCSToken, err = k8int.GetSecret(
		ctx,
		kubeinteraction.GetSecretOpt{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.WebvcsAPISecret.Name,
			Key:       key,
		},
	)

	if err != nil {
		return err
	}
	cs.Info.Pac.VCSInfoFromRepo = true

	cs.Clients.Log.Infof("Using webvcs: url=%s user=%s token-secret=%s in token-key=%s", repo.Spec.WebvcsAPIURL, repo.Spec.WebvcsAPIUser, repo.Spec.WebvcsAPISecret.Name, key)
	return nil
}
