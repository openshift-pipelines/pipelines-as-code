//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/configmap"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGiteaParamsStandardCheckForPushAndPullEvent(t *testing.T) {
	var (
		repoURL      string
		sourceURL    string
		sourceBranch string
		targetBranch string
	)
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request, push",
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun-standard-params-display.yaml",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()
	merged, resp, err := topts.GiteaCNX.Client.MergePullRequest(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index,
		gitea.MergePullRequestOption{
			Title: "Merged with Panache",
			Style: "merge",
		},
	)
	assert.NilError(t, err)
	assert.Assert(t, resp.StatusCode < 400, resp)
	assert.Assert(t, merged)
	tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha, "", false)
	time.Sleep(5 * time.Second)

	// get standard parameter info for pull_request
	_, _, sourceBranch, targetBranch = tgitea.GetStandardParams(t, topts, "pull_request")
	// sourceBranch and targetBranch are different for pull_request
	if sourceBranch == targetBranch {
		assert.Error(t, fmt.Errorf(`source_branch %s is same as target_branch %s for pull_request`, sourceBranch, targetBranch), fmt.Sprintf(`source_branch %s should be different from target_branch %s for pull_request`, sourceBranch, targetBranch))
	}

	// get standard parameter info for push
	repoURL, sourceURL, sourceBranch, targetBranch = tgitea.GetStandardParams(t, topts, "push")
	// sourceBranch and targetBranch are same for push
	if sourceBranch != targetBranch {
		assert.Error(t, fmt.Errorf(`source_branch %s is different from target_branch %s for push`, sourceBranch, targetBranch), fmt.Sprintf(`source_branch %s is same as target_branch %s for push`, sourceBranch, targetBranch))
	}
	// sourceURL and repoURL are same for push
	if repoURL != sourceURL {
		assert.Error(t, fmt.Errorf(`source_url %s is different from repo_url %s for push`, repoURL, sourceURL), fmt.Sprintf(`source_url %s is same as repo_url %s for push`, repoURL, sourceURL))
	}
}

func TestGiteaParamsOnRepoCRWithCustomConsole(t *testing.T) {
	t.Skip("Skipping test changing the global config map for now")
	ctx := context.Background()
	topts := &tgitea.TestOpts{
		CheckForStatus:  "success",
		SkipEventsCheck: true,
		TargetEvent:     options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/params.yaml",
		},
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "custom",
				Value: "myconsole",
			},
		},
		StatusOnlyLatest: true,
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, _ = tgitea.Setup(ctx)
	assert.NilError(t, topts.ParamsRun.Clients.NewClients(ctx, &topts.ParamsRun.Info))
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))

	cfgMapData := map[string]string{
		"custom-console-name":           "Custom Console",
		"custom-console-url":            "https://url",
		"custom-console-url-pr-details": "https://url/detail/{{ custom }}",
		"custom-console-url-pr-tasklog": "https://url/log/{{ custom }}",
		"tekton-dashboard-url":          "",
	}
	defer configmap.ChangeGlobalConfig(ctx, t, topts.ParamsRun, cfgMapData)()
	defer tgitea.TestPR(t, topts)()
	// topts.Regexp = regexp.MustCompile(`(?m).*Custom Console.*https://url/detail/myconsole.*https://url/log/myconsole`)
	topts.Regexp = regexp.MustCompile(`(?m).*Custom Console.*https://url/detail/myconsole`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
	topts.Regexp = regexp.MustCompile(`(?m).*https://url/log/myconsole`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

func TestGiteaParamsOnRepoCR(t *testing.T) {
	topts := &tgitea.TestOpts{
		CheckForStatus:  "success",
		SkipEventsCheck: true,
		TargetEvent:     options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/params.yaml",
		},
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "no_filter",
				Value: "Follow me on my ig #nofilter",
			},
			{
				Name:   "event_type_match",
				Value:  "I am the most Kawaī params",
				Filter: `pac.event_type == "pull_request"`,
			},
			{
				Name:   "event_type_match",
				Value:  "Nobody should see me, i am superseded by the previous params with same name 😠",
				Filter: `pac.event_type == "pull_request"`,
			},
			{
				Name:   "no_match",
				Value:  "Am I being ignored?",
				Filter: `pac.event_type == "xxxxxxx"`,
			},
			{
				Name:   "filter_on_body",
				Value:  "Hey I show up from a payload match",
				Filter: `body.action == "opened"`,
			},
			{
				Name: "secret value",
				SecretRef: &v1alpha1.Secret{
					Name: "param-secret",
					Key:  "secret",
				},
			},
			{
				Name: "secret_nothere",
				SecretRef: &v1alpha1.Secret{
					Name: "param-secret-not-present",
					Key:  "unknowsecret",
				},
			},
		},
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName

	ctx := context.Background()
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, _ = tgitea.Setup(ctx)
	assert.NilError(t, topts.ParamsRun.Clients.NewClients(ctx, &topts.ParamsRun.Info))
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS,
		"param-secret"))

	defer tgitea.TestPR(t, topts)()

	repo, err := topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(repo.Status) != 0)
	assert.NilError(t,
		twait.RegexpMatchingInPodLog(context.Background(),
			topts.ParamsRun,
			topts.TargetNS,
			fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=params",
				repo.Status[0].PipelineRunName),
			"step-test-params-value", *regexp.MustCompile(
				"I am the most Kawaī params\nSHHHHHHH\nFollow me on my ig #nofilter\n{{ no_match }}\nHey I show up from a payload match\n{{ secret_nothere }}"), 2))
}

// TestParamshBodyHeadersCEL Test that we can access the body and headers in params as a CEL expression
func TestParamshBodyHeadersCEL(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request, push",
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun-cel-params.yaml",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()

	repo, err := topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(repo.Status) != 0)

	output := `Look mum I know that we are acting on a pull_request
my email is a true beauty and like groot, I AM pac`
	err = twait.RegexpMatchingInPodLog(context.Background(),
		topts.ParamsRun,
		topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=cel-params",
			repo.Status[0].PipelineRunName),
		"step-test-cel-params-value", *regexp.MustCompile(output), 2)
	assert.NilError(t, err)
}
