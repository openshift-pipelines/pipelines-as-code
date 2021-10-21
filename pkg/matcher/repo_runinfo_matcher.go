package matcher

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gobwas/glob"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func branchMatch(prunBranch, baseBranch string) bool {
	// If we have targetBranch in annotation and refs/heads/targetBranch from
	// webhook, then allow it.
	if filepath.Base(baseBranch) == prunBranch {
		return true
	}

	// match globs like refs/tags/0.*
	g := glob.MustCompile(prunBranch)
	return g.Match(baseBranch)
}

func GetRepoByCR(ctx context.Context, cs *params.Run, ns string) (*apipac.Repository, error) {
	repositories, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	matches := []string{}
	for i := len(repositories.Items) - 1; i >= 0; i-- {
		repo := repositories.Items[i]
		matches = append(matches,
			fmt.Sprintf("RepositoryValue: URL=%s, eventType=%s BaseBranch=%s", repo.Spec.URL,
				repo.Spec.EventType, repo.Spec.Branch))

		if repo.Spec.URL == cs.Info.Event.URL &&
			repo.Spec.EventType == cs.Info.Event.EventType {
			if repo.Spec.Branch != cs.Info.Event.BaseBranch {
				if !branchMatch(repo.Spec.Branch, cs.Info.Event.BaseBranch) {
					continue
				}
			}
			return &repo, nil
		}
	}
	for _, value := range matches {
		cs.Clients.Log.Debug(value)
	}

	return nil, nil
}
