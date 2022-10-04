//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
)

var (
	incomingSecreteValue = "shhhh-secrete"
	incomingSecretName   = "incoming-webhook-secret"
)

func TestGitlabIncomingWebhook(t *testing.T) {
	randomedString := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, glprovider, err := tgitlab.Setup(ctx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")
	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
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

	err = tgitlab.CreateCRD(ctx, projectinfo, runcnx, randomedString, incoming)
	assert.NilError(t, err)

	err = secret.Create(ctx, runcnx, map[string]string{"incoming": incomingSecreteValue}, randomedString, incomingSecretName)
	assert.NilError(t, err)

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pr.yaml": "testdata/pipelinerun.yaml", ".tekton/pr-clone.yaml": "testdata/pipelinerun-clone.yaml",
	}, randomedString, randomedString, options.PushEvent, map[string]string{})
	assert.NilError(t, err)

	title := "TestIncomingWebhook - " + randomedString
	err = tgitlab.PushFilesToRef(glprovider.Client, title,
		projectinfo.DefaultBranch,
		randomedString,
		opts.ProjectID,
		entries, ".tekton/subdir/pr.yaml")
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", randomedString)

	url := fmt.Sprintf("%s/incoming?repository=%s&branch=%s&pipelinerun=%s&secret=%s", opts.ControllerURL,
		randomedString, randomedString, "pipeline-clone", incomingSecreteValue)
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	assert.NilError(t, err)
	httpResp, err := client.Do(req)
	assert.NilError(t, err)
	defer httpResp.Body.Close()
	runcnx.Clients.Log.Infof("Kicked off on incoming-webhook URL: %s", url)
	assert.Assert(t, httpResp.StatusCode > 200 && httpResp.StatusCode < 300)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, -1, randomedString, randomedString, opts.ProjectID)

	wait.Succeeded(ctx, t, runcnx, opts, "Merge_Request", randomedString, 2, "", title)
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGitlabIncomingWebhook$ ."
// End:
