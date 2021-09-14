//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	defaultTimeout   = 10 * time.Minute
	mainBranch       = "main"
	pullRequestEvent = "pull_request"
)

type E2EOptions struct {
	Repo, Owner string
}

func tearDown(ctx context.Context, t *testing.T, cs *cli.Clients, prNumber int, ref string, targetNS string, opts E2EOptions) {
	cs.Log.Infof("Closing PR %d", prNumber)
	if prNumber != -1 {
		state := "closed"
		_, _, err := cs.GithubClient.Client.PullRequests.Edit(ctx,
			opts.Owner, opts.Repo, prNumber,
			&github.PullRequest{State: &state})
		if err != nil {
			t.Fatal(err)
		}
	}

	cs.Log.Infof("Deleting NS %s", targetNS)
	err := cs.Kube.CoreV1().Namespaces().Delete(ctx, targetNS, metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	cs.Log.Infof("Deleting Ref %s", ref)
	_, err = cs.GithubClient.Client.Git.DeleteRef(ctx, opts.Owner, opts.Repo, ref)
	if err != nil {
		t.Fatal(err)
	}
}

func setup() (*cli.Clients, E2EOptions, error) {
	githubURL := os.Getenv("TEST_GITHUB_API_URL")
	githubToken := os.Getenv("TEST_GITHUB_TOKEN")
	githubRepoOwner := os.Getenv("TEST_GITHUB_REPO_OWNER")

	for _, value := range []string{
		"EL_URL", "GITHUB_API_URL", "GITHUB_TOKEN",
		"GITHUB_REPO_OWNER", "EL_WEBHOOK_SECRET",
	} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return nil, E2EOptions{}, fmt.Errorf("\"TEST_%s\" env variable is required, cannot continue", value)
		}
	}

	if githubURL == "" || githubToken == "" || githubRepoOwner == "" {
		return nil, E2EOptions{}, fmt.Errorf("TEST_GITHUB_API_URL TEST_GITHUB_TOKEN TEST_GITHUB_REPO_OWNER need to be set")
	}

	splitted := strings.Split(githubRepoOwner, "/")
	webvcs := webvcs.NewGithubVCS(githubToken, githubURL)

	p := cli.PacParams{}
	cs, err := p.Clients()
	if err != nil {
		return nil, E2EOptions{}, err
	}
	cs.GithubClient = webvcs
	return cs, E2EOptions{Owner: splitted[0], Repo: splitted[1]}, nil
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UTC().UnixNano())
	v := m.Run()
	os.Exit(v)
}
