package pipelineascode

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

// secretFromRepository grab the secret from the repository CRD
func secretFromRepository(ctx context.Context, cs *params.Run, k8int kubeinteraction.Interface, config *info.VCSConfig, repo *apipac.Repository) error {
	var err error

	if repo.Spec.WebvcsAPIURL == "" {
		repo.Spec.WebvcsAPIURL = config.APIURL
	} else {
		cs.Info.Pac.VCSAPIURL = repo.Spec.WebvcsAPIURL
		cs.Clients.Log.Infof("Using WebVCS: %s with api-url: %s", cs.Info.Pac.WebhookType, repo.Spec.WebvcsAPIURL)
	}

	cs.Info.Pac.VCSUser = repo.Spec.WebvcsAPIUser
	cs.Info.Pac.VCSToken, err = k8int.GetSecret(
		ctx,
		kubeinteraction.GetSecretOpt{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.WebvcsAPISecret.Name,
			Key:       repo.Spec.WebvcsAPISecret.Key,
		},
	)

	if err != nil {
		return err
	}
	cs.Info.Pac.VCSInfoFromRepo = true

	if repo.Spec.WebvcsAPIUser != "" {
		cs.Clients.Log.Infof("Using api-user %s", repo.Spec.WebvcsAPIUser)
	}
	cs.Clients.Log.Infof("Using api-token from secret %s in key %s", repo.Spec.WebvcsAPISecret.Name, repo.Spec.WebvcsAPISecret.Key)
	return nil
}
