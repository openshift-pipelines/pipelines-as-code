//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
)

func TestGithubPush(t *testing.T) {
	if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
		t.Skip("Skipping test since only enabled for nightly")
	}
	ctx := context.Background()
	if os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
		g := &tgithub.PRTest{
			Label:            "Github push request on Webhook",
			YamlFiles:        []string{"testdata/pipelinerun-on-push.yaml"},
			SecondController: false,
			Webhook:          true,
		}
		g.RunPushRequest(ctx, t)
		defer g.TearDown(ctx, t)
	} else {
		t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
	}
	g := &tgithub.PRTest{
		Label:     "Github apps push request",
		YamlFiles: []string{"testdata/pipelinerun-on-push.yaml"},
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubSecondPush(t *testing.T) {
	ctx := context.Background()
	g := &tgithub.PRTest{
		Label:            "Github push request",
		YamlFiles:        []string{"testdata/pipelinerun-on-push.yaml"},
		SecondController: true,
	}
	g.RunPushRequest(ctx, t)
	defer g.TearDown(ctx, t)
}

func TestGithubPushRequestCELMatchOnTitle(t *testing.T) {
	ctx := context.Background()
	for _, onWebhook := range []bool{false, true} {
		if onWebhook && os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK") == "" {
			t.Skip("TEST_GITHUB_REPO_OWNER_WEBHOOK is not set")
			continue
		}
		g := &tgithub.PRTest{
			Label:     fmt.Sprintf("Github push request test CEL match on title onWebhook=%v", onWebhook),
			YamlFiles: []string{"testdata/pipelinerun-cel-annotation-for-title-match.yaml"},
			Webhook:   onWebhook,
		}
		g.RunPushRequest(ctx, t)
		defer g.TearDown(ctx, t)
	}
}
