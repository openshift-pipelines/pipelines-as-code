//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-github/v59/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubPullRequestOkToTest(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.TODO()
	g := &tgithub.PRTest{
		Label:     "Github OkToTest comment",
		YamlFiles: []string{"testdata/pipelinerun.yaml"},
	}
	g.RunPullRequest(ctx, t)
	defer g.TearDown(ctx, t)

	repoinfo, resp, err := g.Provider.Client.Repositories.Get(ctx, g.Options.Organization, g.Options.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", g.Options.Organization, g.Options.Repo)
	}

	runevent := info.Event{
		DefaultBranch: repoinfo.GetDefaultBranch(),
		Organization:  g.Options.Organization,
		Repository:    g.Options.Repo,
		URL:           repoinfo.GetHTMLURL(),
		Sender:        g.Options.Organization,
	}

	installID, err := strconv.ParseInt(os.Getenv("TEST_GITHUB_REPO_INSTALLATION_ID"), 10, 64)
	assert.NilError(t, err)
	event := github.IssueCommentEvent{
		Comment: &github.IssueComment{
			Body: github.String(`/ok-to-test`),
		},
		Installation: &github.Installation{
			ID: &installID,
		},
		Action: github.String("created"),
		Issue: &github.Issue{
			State: github.String("open"),
			PullRequestLinks: &github.PullRequestLinks{
				HTMLURL: github.String(fmt.Sprintf("%s/%s/pull/%d",
					os.Getenv("TEST_GITHUB_API_URL"),
					os.Getenv("TEST_GITHUB_REPO_OWNER"), g.PRNumber)),
			},
		},
		Repo: &github.Repository{
			DefaultBranch: &runevent.DefaultBranch,
			HTMLURL:       &runevent.URL,
			Name:          &runevent.Repository,
			Owner:         &github.User{Login: &runevent.Organization},
		},
		Sender: &github.User{
			Login: &runevent.Sender,
		},
	}

	err = payload.Send(ctx,
		g.Cnx,
		os.Getenv("TEST_EL_URL"),
		os.Getenv("TEST_EL_WEBHOOK_SECRET"),
		os.Getenv("TEST_GITHUB_API_URL"),
		os.Getenv("TEST_GITHUB_REPO_INSTALLATION_ID"),
		event,
		"issue_comment",
	)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Wait for the second repository update to be updated")
	waitOpts := twait.Opts{
		RepoName:        g.TargetNamespace,
		Namespace:       g.TargetNamespace,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       g.SHA,
	}
	_, err = twait.UntilRepositoryUpdated(ctx, g.Cnx.Clients, waitOpts)
	assert.NilError(t, err)

	g.Cnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := g.Cnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(g.TargetNamespace).Get(ctx, g.TargetNamespace, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}
