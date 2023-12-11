package matcher

import (
	"context"
	"strings"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MatchEventURLRepo(ctx context.Context, cs *params.Run, event *info.Event, ns string) (*apipac.Repository, error) {
	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := len(repositories.Items) - 1; i >= 0; i-- {
		repo := repositories.Items[i]
		repo.Spec.URL = strings.TrimSuffix(repo.Spec.URL, "/")
		if repo.Spec.URL == event.URL {
			return &repo, nil
		}
	}

	return nil, nil
}

// GetRepo get a repo by name anywhere on a cluster.
func GetRepo(ctx context.Context, cs *params.Run, repoName string) (*apipac.Repository, error) {
	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(
		ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := len(repositories.Items) - 1; i >= 0; i-- {
		repo := repositories.Items[i]
		if repo.GetName() == repoName {
			return &repo, nil
		}
	}

	return nil, nil
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
