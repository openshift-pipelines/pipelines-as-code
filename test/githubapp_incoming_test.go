//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGithubAppIncoming tests that a Pipelinerun with the incoming event
// gets created despite the presence of multiple Pipelineruns in the .tekton directory with
// eventType as incoming
func TestGithubAppIncoming(t *testing.T) {
	randomedString := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun-incoming.yaml": "testdata/pipelinerun-incoming.yaml", ".tekton/pr.yaml": "testdata/pipelinerun.yaml",
	}, randomedString, randomedString, options.IncomingEvent, map[string]string{})
	assert.NilError(t, err)

	verifyIncomingWebhook(t, randomedString, entries)
}

// TestGithubAppIncomingForDifferentEvent tests that a Pipelinerun with the incoming event
// gets created despite the presence of multiple Pipelineruns in the .tekton directory,
// where one has an eventType as incoming and another as pull_request
func TestGithubAppIncomingForDifferentEvent(t *testing.T) {
	randomedString := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")

	entries, err := payload.GetEntries(map[string]string{
		".tekton/pipelinerun-incoming.yaml": "testdata/pipelinerun-incoming-generatename.yaml", ".tekton/pr.yaml": "testdata/pipelinerun.yaml",
	}, randomedString, randomedString, options.PullRequestEvent, map[string]string{})
	assert.NilError(t, err)

	verifyIncomingWebhook(t, randomedString, entries)
}

func verifyIncomingWebhook(t *testing.T, randomedString string, entries map[string]string) {
	ctx := context.Background()
	runcnx, opts, ghprovider, err := tgithub.Setup(ctx, false)
	assert.NilError(t, err)
	label := "GithubApp Incoming"
	logmsg := fmt.Sprintf("Testing %s with Github APPS integration on %s", label, randomedString)
	runcnx.Clients.Log.Info(logmsg)

	repoinfo, resp, err := ghprovider.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
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
			Params: []string{
				"the_best_superhero_is",
			},
		},
	}

	err = tgithub.CreateCRDIncoming(ctx, t, repoinfo, runcnx, incoming, opts, randomedString)
	assert.NilError(t, err)

	err = secret.Create(ctx, runcnx, map[string]string{"incoming": incomingSecreteValue}, randomedString, incomingSecretName)
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
		randomedString, randomedString, "pipelinerun-incoming", incomingSecreteValue)
	body := `{"params":{"the_best_superhero_is":"Superman"}}`
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	req.Header.Add("Content-Type", "application/json")
	assert.NilError(t, err)
	httpResp, err := client.Do(req)
	assert.NilError(t, err)
	defer httpResp.Body.Close()
	runcnx.Clients.Log.Infof("Kicked off on incoming URL: %s", url)
	assert.Assert(t, httpResp.StatusCode >= 200 && httpResp.StatusCode < 300)
	// to re enable after debugging...
	defer tgithub.TearDown(ctx, t, runcnx, ghprovider, -1, targetRefName, randomedString, opts)

	sopt := wait.SuccessOpt{
		Title:           title,
		OnEvent:         options.IncomingEvent,
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

	err = wait.RegexpMatchingInPodLog(context.Background(),
		runcnx, randomedString,
		"pipelinesascode.tekton.dev/event-type=incoming",
		"step-task", *regexp.MustCompile(".*It's a Bird... It's a Plane... It's Superman"), 2)
	assert.NilError(t, err, "Error while checking the logs of the pods")
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGithubAppIncoming ."
// End:
