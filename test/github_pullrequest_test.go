//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v59/github"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestGithubPullRequest(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github PullRequest",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestSecondController(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github Rerequest",
		YamlFiles:        []string{"testdata/pipelinerun.yaml"},
		SecondController: true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestMultiples(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github multiple PullRequest",
		YamlFiles: []string{"testdata/pipelinerun.yaml", "testdata/pipelinerun-clone.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestMatchOnCEL(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github CEL Match",
		YamlFiles: []string{"testdata/pipelinerun-cel-annotation.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestCELMatchOnTitle(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:     "Github CEL Match on Title",
		YamlFiles: []string{"testdata/pipelinerun-cel-annotation-for-title-match.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestWebhook(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
		return
	}
	ctx := context.Background()

	g := &tgithub.PRTest{
		Label:     "Github PullRequest onWebhook",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
		Webhook:   true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPullRequestSecondBadYaml(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github Rerequest",
		YamlFiles:        []string{"testdata/failures/bad-yaml.yaml"},
		SecondController: true,
		NoStatusCheck:    true,
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	opt := github.ListOptions{}
	res := &github.ListCheckRunsResults{}
	resp := &github.Response{}
	var err error
	counter := 0
	for {
		res, resp, err = g.Provider.Client.Checks.ListCheckRunsForRef(ctx, g.Options.Organization, g.Options.Repo, g.SHA, &github.ListCheckRunsOptions{
			AppID:       g.Provider.ApplicationID,
			ListOptions: opt,
		})
		assert.NilError(t, err)
		assert.Equal(t, resp.StatusCode, 200)
		if len(res.CheckRuns) > 0 {
			break
		}
		g.Cnx.Clients.Log.Infof("Waiting for the check run to be created")
		if counter > 10 {
			t.Errorf("Check run not created after 10 tries")
			break
		}
		time.Sleep(5 * time.Second)
	}
	assert.Equal(t, len(res.CheckRuns), 1)
	assert.Equal(t, res.CheckRuns[0].GetOutput().GetTitle(), "Failed")
	// may be fragile if we change the application name, but life goes on if it fails and we fix the name if that happen
	assert.Equal(t, res.CheckRuns[0].GetOutput().GetSummary(), "Pipelines as Code GHE has <b>failed</b>.")
	golden.Assert(t, res.CheckRuns[0].GetOutput().GetText(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -info TestGithubPullRequest$ ."
// End:
