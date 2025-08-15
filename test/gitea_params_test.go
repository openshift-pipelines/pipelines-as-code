//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/configmap"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
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
	_, f := tgitea.TestPR(t, topts)
	defer f()
	merged, resp, err := topts.GiteaCNX.Client().MergePullRequest(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index,
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
		TargetEvent:     triggertype.PullRequest.String(),
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
	ctx, err := cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))

	cfgMapData := map[string]string{
		"custom-console-name":           "Custom Console",
		"custom-console-url":            "https://url",
		"custom-console-url-pr-details": "https://url/detail/{{ custom }}",
		"custom-console-url-pr-tasklog": "https://url/log/{{ custom }}",
		"tekton-dashboard-url":          "",
	}
	defer configmap.ChangeGlobalConfig(ctx, t, topts.ParamsRun, cfgMapData)()
	_, f := tgitea.TestPR(t, topts)
	defer f()
	// topts.Regexp = regexp.MustCompile(`(?m).*Custom Console.*https://url/detail/myconsole.*https://url/log/myconsole`)
	topts.Regexp = regexp.MustCompile(`(?m).*Custom Console.*https://url/detail/myconsole`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
	topts.Regexp = regexp.MustCompile(`(?m).*https://url/log/myconsole`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

func TestGiteaGlobalRepoParams(t *testing.T) {
	topts := &tgitea.TestOpts{
		CheckForStatus:  "success",
		SkipEventsCheck: true,
		TargetEvent:     triggertype.PullRequest.String(),
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/params.yaml",
		},
		GlobalRepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "no_filter",
				Value: "I come from the global params",
			},
		},
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName
	ctx := context.Background()
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, _ = tgitea.Setup(ctx)
	assert.NilError(t, topts.ParamsRun.Clients.NewClients(ctx, &topts.ParamsRun.Info))
	ctx, err := cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	_, f := tgitea.TestPR(t, topts)
	defer f()

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
	}
	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	last := repo.Status[len(repo.Status)-1]
	err = twait.RegexpMatchingInPodLog(
		context.Background(),
		topts.ParamsRun,
		topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", last.PipelineRunName),
		"step-test-params-value",
		regexp.Regexp{},
		t.Name(),
		2,
	)
	assert.NilError(t, err)
}

// TestGiteaGlobalRepoUseLocalDef will test when having params from the global
// and local repository or gitprovider secret on both it uses the local first.
func TestGiteaGlobalRepoUseLocalDef(t *testing.T) {
	topts := &tgitea.TestOpts{
		CheckForStatus:  "success",
		SkipEventsCheck: true,
		TargetEvent:     triggertype.PullRequest.String(),
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/params.yaml",
		},
		RepoCRParams: &[]v1alpha1.Params{
			{
				Name:  "no_filter",
				Value: "I come from the local params",
			},
		},
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName
	ctx := context.Background()

	topts.ParamsRun, topts.Opts, topts.GiteaCNX, _ = tgitea.Setup(ctx)
	assert.NilError(t, topts.ParamsRun.Clients.NewClients(ctx, &topts.ParamsRun.Info))
	ctx, err := cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))

	globalNs := info.GetNS(ctx)
	err = tgitea.CreateCRD(ctx, topts,
		v1alpha1.RepositorySpec{
			GitProvider: &v1alpha1.GitProvider{
				Secret: &v1alpha1.Secret{
					Name: "notreallyhere",
				},
			},
			Params: &[]v1alpha1.Params{
				{
					Name:  "no_filter",
					Value: "I come from the global params",
				},
			},
		},
		true)
	assert.NilError(t, err)

	defer (func() {
		if os.Getenv("TEST_NOCLEANUP") != "true" {
			topts.ParamsRun.Clients.Log.Infof("Cleaning up global repo %s in %s", info.DefaultGlobalRepoName, globalNs)
			_ = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(globalNs).Delete(
				context.Background(), info.DefaultGlobalRepoName, metav1.DeleteOptions{})
		}
	})()

	_, f := tgitea.TestPR(t, topts)
	defer f()

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
	}
	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	last := repo.Status[len(repo.Status)-1]
	err = twait.RegexpMatchingInPodLog(
		context.Background(),
		topts.ParamsRun,
		topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s", last.PipelineRunName),
		"step-test-params-value",
		regexp.Regexp{},
		t.Name(),
		2,
	)
	assert.NilError(t, err)
}

func TestGiteaParamsOnRepoCR(t *testing.T) {
	topts := &tgitea.TestOpts{
		CheckForStatus:  "success",
		SkipEventsCheck: true,
		TargetEvent:     triggertype.PullRequest.String(),
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
			{
				Name: "no_initial_value",
			},
		},
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName

	ctx := context.Background()
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, _ = tgitea.Setup(ctx)
	assert.NilError(t, topts.ParamsRun.Clients.NewClients(ctx, &topts.ParamsRun.Info))
	ctx, err := cctx.GetControllerCtxInfo(ctx, topts.ParamsRun)
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS,
		"param-secret"))

	_, f := tgitea.TestPR(t, topts)
	defer f()

	// Wait for Repository status to be updated
	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       "",
	}
	repo, err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	assert.Assert(t, len(repo.Status) != 0)
	assert.NilError(t,
		twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=params",
			repo.Status[0].PipelineRunName), "step-test-params-value", *regexp.MustCompile(
			"I am the most Kawaī params\nSHHHHHHH\nFollow me on my ig #nofilter\n{{ no_match }}\nHey I show up from a payload match\n{{ secret_nothere }}\n{{ no_initial_value }}"), "", 2))
}

// TestGiteaParamsBodyHeadersCEL Test that we can access the pull request body and headers in params
// as a CEL expression and cel filter.
func TestGiteaParamsBodyHeadersCEL(t *testing.T) {
	// Setup a repo and create a pull request with two pipelinerun in tekton
	// dir, one matching pull via cel filtering expression and one for push
	// and make it succeed
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request",
		YAMLFiles: map[string]string{
			".tekton/pullrequest.yaml": "testdata/pipelinerun-cel-params-pullrequest.yaml",
			".tekton/push.yaml":        "testdata/pipelinerun-cel-params-push.yaml",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	_, f := tgitea.TestPR(t, topts)
	defer f()

	// check the repos CR only one pr should have run
	repo, err := topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 1, repo.Status)

	// check the output logs if the CEL body headers has expanded  properly
	output := `Look mum I know that we are acting on a pull_request
my email is a true beauty and like groot, I AM pac`
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=cel-pullrequest-params",
		repo.Status[0].PipelineRunName), "step-test-cel-params-value", *regexp.MustCompile(output), "", 2)
	assert.NilError(t, err)

	// Merge the pull request so we can generate a push event and wait that it is updated
	merged, resp, err := topts.GiteaCNX.Client().MergePullRequest(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index,
		gitea.MergePullRequestOption{
			Title: "Merged with Panache",
			Style: "merge",
		},
	)
	assert.NilError(t, err)
	assert.Assert(t, resp.StatusCode < 400, resp)
	assert.Assert(t, merged)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 2, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	_, err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(5 * time.Second)

	// check the repository CR now we should have two status the previous pull request and new one on push
	repo, err = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 2, repo.Status)

	// sort status to make sure we get the latest PipelineRun that has been created
	sortedstatus := sort.RepositorySortRunStatus(repo.Status)

	// check the output of the last status PipelineRun which should be a
	// push matching the expanded CEL body and headers values
	output = `Look mum I know that we are acting on a push
my email is a true beauty and you can call me pacman`
	err = twait.RegexpMatchingInPodLog(context.Background(), topts.ParamsRun, topts.TargetNS, fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=cel-push-params", sortedstatus[0].PipelineRunName), "step-test-cel-params-value", *regexp.MustCompile(output), "", 2)
	assert.NilError(t, err)
}

// TestGiteaParamsChangedFilesCEL Test that we can access the pull request body and headers in params
// as a CEL expression and cel filter.
func TestGiteaParamsChangedFilesCEL(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request",
		YAMLFiles: map[string]string{
			".tekton/pullrequest.yaml": "testdata/pipelinerun-changed-files-pullrequest.yaml",
			".tekton/push.yaml":        "testdata/pipelinerun-changed-files-push.yaml",
			"deleted.txt":              "testdata/changed_files_deleted",
			"modified.txt":             "testdata/changed_files_modified",
			"renamed.txt":              "testdata/changed_files_renamed",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
		FileChanges: []scm.FileChange{
			{
				FileName:   "deleted.txt",
				ChangeType: "delete",
			},
			{
				FileName:   "modified.txt",
				ChangeType: "modify",
				NewContent: "this file has been modified",
			},
			{
				FileName:   "renamed.txt",
				ChangeType: "rename",
				NewName:    "hasbeenrenamed.txt",
			},
		},
	}

	_, f := tgitea.TestPR(t, topts)
	defer f()

	// check the repos CR only one pr should have run
	repo, err := topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 1, repo.Status)
	twait.GoldenPodLog(context.Background(), t, topts.ParamsRun, topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=changed-files-pullrequest-params", repo.Status[0].PipelineRunName),
		"step-test-changed-files-params-pull", strings.ReplaceAll(fmt.Sprintf("%s-changed-files-pullrequest-params-1.golden", t.Name()), "/", "-"), 2)
	// ======================================================================================================================
	// Merge the pull request so we can generate a push event and wait that it is updated
	// ======================================================================================================================
	merged, resp, err := topts.GiteaCNX.Client().MergePullRequest(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index,
		gitea.MergePullRequestOption{
			Title: "Merged with Panache",
			Style: "merge",
		},
	)
	assert.NilError(t, err)
	assert.Assert(t, resp.StatusCode < 400, resp)
	assert.Assert(t, merged)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 2, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	_, err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	time.Sleep(5 * time.Second)

	// check the repository CR now we should have two status the previous pull request and new one on push
	repo, err = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 2, repo.Status)
	// sort status to make sure we get the latest PipelineRun that has been created
	sortedstatus := sort.RepositorySortRunStatus(repo.Status)

	twait.GoldenPodLog(context.Background(), t, topts.ParamsRun, topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=changed-files-push-params", sortedstatus[0].PipelineRunName),
		"step-test-changed-files-params-push", strings.ReplaceAll(fmt.Sprintf("%s-changed-files-push-params-1.golden", t.Name()), "/", "-"), 2)

	// ======================================================================================================================
	// Create second pull request with all change types
	// ======================================================================================================================
	tgitea.NewPR(t, topts)
	// check the repository CR now we should have three status the previous pull request and push plus a new pull request
	repo, err = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 3, repo.Status)
	twait.GoldenPodLog(context.Background(), t, topts.ParamsRun, topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=changed-files-pullrequest-params", repo.Status[2].PipelineRunName),
		"step-test-changed-files-params-pull", strings.ReplaceAll(fmt.Sprintf("%s-changed-files-pullrequest-params-2.golden", t.Name()), "/", "-"), 2)

	// ======================================================================================================================
	// Merge the pull request so we can generate a second push event and wait that it is updated
	// ======================================================================================================================
	merged, resp, err = topts.GiteaCNX.Client().MergePullRequest(topts.Opts.Organization, topts.Opts.Repo, topts.PullRequest.Index,
		gitea.MergePullRequestOption{
			Title: "Merged with Panache",
			Style: "merge",
		},
	)
	assert.NilError(t, err)
	assert.Assert(t, resp.StatusCode < 400, resp)
	assert.Assert(t, merged)

	waitOpts = twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 4, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	_, err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
	time.Sleep(5 * time.Second)

	// check the repository CR now we should have two status the previous pull request and new one on push
	repo, err = topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(repo.Status), 4, repo.Status)
	// sort status to make sure we get the latest PipelineRun that has been created
	sortedstatus = sort.RepositorySortRunStatus(repo.Status)
	// check the output of the last status PipelineRun which should be a
	// push matching the expanded CEL body and headers values
	twait.GoldenPodLog(context.Background(), t, topts.ParamsRun, topts.TargetNS,
		fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=changed-files-push-params", sortedstatus[0].PipelineRunName),
		"step-test-changed-files-params-push", strings.ReplaceAll(fmt.Sprintf("%s-changed-files-push-params-2.golden", t.Name()), "/", "-"), 2)
}
