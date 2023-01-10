//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

func TestGithubAppIncoming(t *testing.T) {
	randomedString := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, ghprovider, err := tgithub.Setup(ctx, false)
	assert.NilError(t, err)
	label := "GithubApp Incoming"
	logmsg := fmt.Sprintf("Testing %s with Github APPS integration on %s", label, randomedString)
	runcnx.Clients.Log.Info(logmsg)

	repoinfo, resp, err := ghprovider.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	incoming := &[]v1alpha1.Incoming{
		{
			Type: "webhook-url",
			Secret: v1alpha1.Secret{
				Name: incomingSecretName,
				Key:  "incoming",
			},
			Targets: []string{randomedString},
		},
	}

	err = tgithub.CreateCRDIncoming(ctx, t, repoinfo, runcnx, incoming, opts, randomedString)
	assert.NilError(t, err)

	err = secret.Create(ctx, runcnx, map[string]string{"incoming": incomingSecreteValue}, randomedString, incomingSecretName)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pr-clone.yaml": "testdata/pipelinerun-clone.yaml", ".tekton/pr.yaml": "testdata/pipelinerun.yaml",
	}, randomedString, randomedString, options.PushEvent, map[string]string{})
	assert.NilError(t, err)

	targetRefName := fmt.Sprintf("refs/heads/%s", randomedString)

	title := "TestGithubAppIncoming - " + randomedString
	sha, err := tgithub.PushFilesToRef(ctx, ghprovider.Client, title,
		repoinfo.GetDefaultBranch(),
		targetRefName,
		opts.Organization,
		opts.Repo,
		entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to branch %s", sha, targetRefName)

	url := fmt.Sprintf("%s/incoming?repository=%s&branch=%s&pipelinerun=%s&secret=%s", opts.ControllerURL,
		randomedString, randomedString, "pr-clone", incomingSecreteValue)

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	assert.NilError(t, err)
	httpResp, err := client.Do(req)
	assert.NilError(t, err)
	defer httpResp.Body.Close()
	runcnx.Clients.Log.Infof("Kicked off on incoming URL: %s", url)
	assert.Assert(t, httpResp.StatusCode > 200 && httpResp.StatusCode < 300)
	defer tgithub.TearDown(ctx, t, runcnx, ghprovider, -1, targetRefName, randomedString, opts)

	wait.Succeeded(ctx, t, runcnx, opts, "Pull_Request", randomedString, 3, "", title)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGithubAppIncoming$ ."
// End:
