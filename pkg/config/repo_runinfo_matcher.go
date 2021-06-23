package config

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gobwas/glob"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
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

func GetRepoByCR(ctx context.Context, cs *cli.Clients, ns string, runinfo *webvcs.RunInfo) (*apipac.Repository, error) {
	repositories, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(ns).List(
		ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	matches := []string{}
	for _, value := range repositories.Items {
		matches = append(matches,
			fmt.Sprintf("RepositoryValue: URL=%s, eventType=%s BaseBranch:=%s", value.Spec.URL,
				value.Spec.EventType, value.Spec.Branch))

		if value.Spec.URL == runinfo.URL &&
			value.Spec.EventType == runinfo.EventType {
			if value.Spec.Branch != runinfo.BaseBranch {
				if !branchMatch(value.Spec.Branch, runinfo.BaseBranch) {
					continue
				}
			}

			// Disallow attempts for hijacks. If the installed CR is not configured on the
			// namespace the Spec is targeting then disallow it.
			if value.Namespace != value.Spec.Namespace {
				return nil, fmt.Errorf("repo CR %s matches but belongs to %s while it should be in %s",
					value.Name,
					value.Namespace,
					value.Spec.Namespace)
			}

			return &value, nil
		}
	}
	for _, value := range matches {
		cs.Log.Debug(value)
	}

	return nil, nil
}
