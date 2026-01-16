package github

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pacgithub "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"gotest.tools/v3/assert"
)

func CreateTag(ctx context.Context, t *testing.T, runcnx *params.Run, ghcnx *pacgithub.Provider, opts options.E2E, sha, tagName string) (*github.Tag, error) {
	createTag := github.CreateTag{
		Tag:     tagName,
		Message: "Release " + tagName,
		Object:  sha,
		Type:    "commit",
		Tagger: &github.CommitAuthor{
			Name:  github.Ptr("OpenShift Pipelines E2E test"),
			Email: github.Ptr("e2e-pipeline@redhat.com"),
			Date:  &github.Timestamp{Time: time.Now()},
		},
	}
	tag, _, err := ghcnx.Client().Git.CreateTag(ctx, opts.Organization, opts.Repo, createTag)
	assert.NilError(t, err)

	createRef := github.CreateRef{
		Ref: "refs/tags/" + tagName,
		SHA: *tag.SHA,
	}
	_, _, err = ghcnx.Client().Git.CreateRef(ctx, opts.Organization, opts.Repo, createRef)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Tag %s has been created successfully", tag.GetTag())

	return tag, nil
}

func DeleteTag(ctx context.Context, ghcnx *pacgithub.Provider, opts options.E2E, tagName string) error {
	_, err := ghcnx.Client().Git.DeleteRef(ctx, opts.Organization, opts.Repo, "refs/tags/"+tagName)
	return err
}
