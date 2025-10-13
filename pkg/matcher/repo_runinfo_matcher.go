package matcher

import (
	"context"
	"errors"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrRepositoryNameConflict = errors.New("multiple repositories exist with the given name")

func MatchEventURLRepo(ctx context.Context, cs *params.Run, event *info.Event, ns string) (*apipac.Repository, error) {
	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{})
	sort.RepositorySortByCreationOldestTime(repositories.Items)
	if err != nil {
		return nil, err
	}
	for _, repo := range repositories.Items {
		repo.Spec.URL = strings.TrimSuffix(repo.Spec.URL, "/")
		if repo.Spec.URL == event.URL {
			return &repo, nil
		}
	}

	return nil, nil
}

// GetRepoByName get a repo by name anywhere on a cluster.
// Parameter 'ns' may optionally be supplied in case of a naming conflict.
func GetRepoByName(ctx context.Context, cs *params.Run, repoName string, ns string) (*apipac.Repository, error) {
	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{
			FieldSelector: "metadata.name==" + repoName,
		})
	if err != nil {
		return nil, err
	}

	switch len(repositories.Items) {
	case 0:
		return nil, nil
	case 1:
		return &repositories.Items[0], nil
	default:
		return nil, ErrRepositoryNameConflict
	}
}

// IncomingWebhookRule will match a rule to an incoming rule, currently a rule is a target branch.
func IncomingWebhookRule(branch string, incomingWebhooks []apipac.Incoming) *apipac.Incoming {
	// TODO: one day we will match the hook.Type here when we get something else than the dumb one (ie: slack)
	for _, hook := range incomingWebhooks {
		for _, v := range hook.Targets {
			if v == branch {
				return &hook
			}
		}
	}
	return nil
}
