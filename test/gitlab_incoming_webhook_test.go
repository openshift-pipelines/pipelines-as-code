//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	tgitlab "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ctx, err = cctx.GetControllerCtxInfo(ctx, runcnx)
	assert.NilError(t, err)
	runcnx.Clients.Log.Info("Testing with Gitlab")
	projectinfo, resp, err := glprovider.Client.Projects.GetProject(opts.ProjectID, nil)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
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
		".tekton/pipelinerun-incoming.yaml": "testdata/pipelinerun-incoming.yaml",
		".tekton/subdir/pr.yaml":            "testdata/pipelinerun-clone.yaml",
	}, randomedString, randomedString, triggertype.Incoming.String(), map[string]string{})
	assert.NilError(t, err)

	title := "TestIncomingWebhook - " + randomedString
	gitCloneURL, err := scm.MakeGitCloneURL(projectinfo.WebURL, opts.UserName, opts.Password)
	assert.NilError(t, err)
	scmOpts := &scm.Opts{
		GitURL:        gitCloneURL,
		Log:           runcnx.Clients.Log,
		WebURL:        projectinfo.WebURL,
		TargetRefName: randomedString,
		BaseRefName:   projectinfo.DefaultBranch,
		CommitTitle:   title,
	}
	scm.PushFilesToRefGit(t, scmOpts, entries)
	runcnx.Clients.Log.Infof("Branch %s has been created and pushed with files", randomedString)

	url := fmt.Sprintf("%s/incoming?repository=%s&branch=%s&pipelinerun=%s&secret=%s", opts.ControllerURL,
		randomedString, randomedString, "pipelinerun-incoming", incomingSecreteValue)
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	assert.NilError(t, err)
	httpResp, err := client.Do(req)
	assert.NilError(t, err)
	defer httpResp.Body.Close()
	runcnx.Clients.Log.Infof("Kicked off on incoming-webhook URL: %s", url)
	assert.Assert(t, httpResp.StatusCode > 200 && httpResp.StatusCode < 300)
	defer tgitlab.TearDown(ctx, t, runcnx, glprovider, -1, randomedString, randomedString, opts.ProjectID)

	sopt := wait.SuccessOpt{
		Title:           title,
		OnEvent:         triggertype.Incoming.String(),
		TargetNS:        randomedString,
		NumberofPRMatch: 1,
		SHA:             "",
	}
	wait.Succeeded(ctx, t, runcnx, opts, sopt)
	prsNew, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(randomedString).List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(prsNew.Items) == 1)

	prName := prsNew.Items[0].GetName()
	index := strings.LastIndex(prsNew.Items[0].GetGenerateName(), "-")
	if index != -1 {
		prName = prsNew.Items[0].GetGenerateName()[:index]
	}
	assert.Assert(t, prName == "pipelinerun-incoming")
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGitlabIncomingWebhook$ ."
// End:
