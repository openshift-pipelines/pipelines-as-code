package wait

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var DefaultTimeout = 10 * time.Minute

func Succeeded(ctx context.Context, t *testing.T, runcnx *params.Run, opts options.E2E, onEvent, targetNS string, numberofprmatch int, sha, title string) {
	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")
	waitOpts := Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 0,
		PollTimeout:     DefaultTimeout,
		TargetSHA:       sha,
	}
	err := UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, v1.GetOptions{})
	assert.NilError(t, err)

	for i := 1; i == numberofprmatch; i++ {
		laststatus := repo.Status[len(repo.Status)-i]
		assert.Equal(t, corev1.ConditionTrue, laststatus.Conditions[0].Status)
		if sha != "" {
			assert.Equal(t, sha, *laststatus.SHA)
			assert.Equal(t, sha, filepath.Base(*laststatus.SHAURL))
		}
		assert.Equal(t, title, *laststatus.Title)
		assert.Assert(t, *laststatus.LogURL != "")

		pr, err := runcnx.Clients.Tekton.TektonV1beta1().PipelineRuns(targetNS).Get(ctx, laststatus.PipelineRunName, v1.GetOptions{})
		assert.NilError(t, err)

		assert.Equal(t, onEvent, pr.Labels["pipelinesascode.tekton.dev/event-type"])
		assert.Equal(t, repo.GetName(), pr.Labels["pipelinesascode.tekton.dev/repository"])
		// assert.Equal(t, opts.Owner, pr.Labels["pipelinesascode.tekton.dev/sender"]) bitbucket is too weird for that

		if opts.Organization != "" {
			assert.Equal(t, opts.Organization, pr.Labels["pipelinesascode.tekton.dev/url-org"])
		}
		if opts.Repo != "" {
			assert.Equal(t, opts.Repo, pr.Labels["pipelinesascode.tekton.dev/url-repository"])
		}
		if sha != "" {
			assert.Equal(t, sha, pr.Labels["pipelinesascode.tekton.dev/sha"])
			assert.Equal(t, sha, filepath.Base(pr.Annotations["pipelinesascode.tekton.dev/sha-url"]))
		}
		assert.Equal(t, title, pr.Annotations["pipelinesascode.tekton.dev/sha-title"])
	}
}
