package github

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pacgithub "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"gotest.tools/v3/assert"
)

func CreateTag(ctx context.Context, t *testing.T, runcnx *params.Run, ghcnx *pacgithub.Provider, opts options.E2E, sha, tagName string) (*github.Tag, error) {
	tag := &github.Tag{
		Tag:     github.Ptr(tagName),
		Message: github.Ptr("Release " + tagName),
		Object: &github.GitObject{
			SHA:  github.Ptr(sha),
			Type: github.Ptr("commit"),
		},
		Tagger: &github.CommitAuthor{
			Name:  github.Ptr("OpenShift Pipelines E2E test"),
			Email: github.Ptr("e2e-pipeline@redhat.com"),
			Date:  &github.Timestamp{Time: time.Now()},
		},
	}
	tag, _, err := ghcnx.Client().Git.CreateTag(ctx, opts.Organization, opts.Repo, tag)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Tag %s has been created successfully", *tag.SHA)

	refToCreate := &github.Reference{
		Ref:    github.Ptr("refs/tags/" + tagName),
		Object: &github.GitObject{SHA: tag.SHA},
	}
	_, _, err = ghcnx.Client().Git.CreateRef(ctx, opts.Organization, opts.Repo, refToCreate)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Tag %s has been created successfully", tag.GetTag())

	return tag, nil
}
