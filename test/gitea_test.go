//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v53/github"
	pacapi "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tknpacdelete "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/deleterepo"
	tknpacdesc "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/describe"
	tknpacgenerate "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/generate"
	tknpaclist "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/list"
	tknpacresolve "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	tknpactest "github.com/openshift-pipelines/pipelines-as-code/test/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/configmap"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/secret"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var successRegexp = regexp.MustCompile(`^Pipelines as Code CI.*has.*successfully`)

func TestGiteaPullRequestTaskAnnotations(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pipeline.yaml":                        "testdata/pipeline_in_tektondir.yaml",
			".other-tasks/task-referenced-internally.yaml": "testdata/task_referenced_internally.yaml",
			".tekton/pr.yaml":                              "testdata/pipelinerun_remote_task_annotations.yaml",
		},
		CheckForStatus: "success",
		ExtraArgs: map[string]string{
			"RemoteTaskURL":  options.RemoteTaskURL,
			"RemoteTaskName": options.RemoteTaskName,
		},
	}
	defer tgitea.TestPR(t, topts)()
}

func TestGiteaUseDisplayName(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      regexp.MustCompile(`.*The Task name is Task.*`),
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		CheckForStatus: "success",
		ExtraArgs: map[string]string{
			"RemoteTaskURL":  options.RemoteTaskURL,
			"RemoteTaskName": options.RemoteTaskName,
		},
	}
	defer tgitea.TestPR(t, topts)()
}

func TestGiteaPullRequestPipelineAnnotations(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun_remote_pipeline_annotations.yaml",
		},
		ExpectEvents:   false,
		CheckForStatus: "success",
		ExtraArgs: map[string]string{
			"RemoteTaskURL":  options.RemoteTaskURL,
			"RemoteTaskName": options.RemoteTaskName,
		},
	}
	defer tgitea.TestPR(t, topts)()
}

func TestGiteaPullRequestPrivateRepository(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pipeline.yaml": "testdata/pipelinerun_git_clone_private-gitea.yaml",
		},
		ExpectEvents:   false,
		CheckForStatus: "success",
	}
	defer tgitea.TestPR(t, topts)()

	tgitea.WaitForSecretDeletion(t, topts, topts.TargetRefName)
}

// TestGiteaBadYaml we can't check pr status but this shows up in the
// controller, so let's dig ourself in there....  TargetNS is a random string, so
// it can only success if it matches it
func TestGiteaBadYaml(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent:  options.PullRequestEvent,
		YAMLFiles:    map[string]string{".tekton/pr-bad-format.yaml": "testdata/failures/pipeline_bad_format.yaml"},
		ExpectEvents: true,
	}
	defer tgitea.TestPR(t, topts)()
	ctx := context.Background()

	assert.NilError(t, twait.RegexpMatchingInControllerLog(ctx, topts.ParamsRun, *regexp.MustCompile(
		"pipelinerun.*has failed.*expected exactly one, got neither: spec.pipelineRef, spec.pipelineSpec"), 10))
}

// don't test concurrency limit here, just parallel pipeline
func TestGiteaMultiplesParallelPipelines(t *testing.T) {
	maxParallel := 10
	yamlFiles := map[string]string{}
	for i := 0; i < maxParallel; i++ {
		yamlFiles[fmt.Sprintf(".tekton/pr%d.yaml", i)] = "testdata/pipelinerun.yaml"
	}
	topts := &tgitea.TestOpts{
		Regexp:               successRegexp,
		TargetEvent:          options.PullRequestEvent,
		YAMLFiles:            yamlFiles,
		CheckForStatus:       "success",
		CheckForNumberStatus: maxParallel,
		ExpectEvents:         false,
	}
	defer tgitea.TestPR(t, topts)()
}

// multiple pipelineruns in the same .tekton directory and a concurrency of 1
func TestGiteaConcurrencyExclusivenessMultiplePipelines(t *testing.T) {
	numPipelines := 10
	yamlFiles := map[string]string{}
	for i := 0; i < numPipelines; i++ {
		yamlFiles[fmt.Sprintf(".tekton/pr%d.yaml", i)] = "testdata/pipelinerun.yaml"
	}
	topts := &tgitea.TestOpts{
		Regexp:               successRegexp,
		TargetEvent:          options.PullRequestEvent,
		YAMLFiles:            yamlFiles,
		CheckForStatus:       "success",
		CheckForNumberStatus: numPipelines,
		ConcurrencyLimit:     github.Int(1),
		ExpectEvents:         false,
	}
	defer tgitea.TestPR(t, topts)()
}

// multiple push to the same  repo, concurrency should q them
func TestGiteaConcurrencyExclusivenessMultipleRuns(t *testing.T) {
	numPipelines := 1
	topts := &tgitea.TestOpts{
		TargetEvent:          options.PullRequestEvent,
		YAMLFiles:            map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"},
		CheckForNumberStatus: numPipelines,
		ConcurrencyLimit:     github.Int(1),
		NoCleanup:            true,
		ExpectEvents:         false,
	}
	defer tgitea.TestPR(t, topts)()
	processed, err := payload.ApplyTemplate("testdata/pipelinerun-alt.yaml", map[string]string{
		"TargetNamespace": topts.TargetNS,
		"TargetBranch":    topts.DefaultBranch,
		"TargetEvent":     topts.TargetEvent,
		"PipelineName":    "pr",
		"Command":         "sleep 10",
	})
	assert.NilError(t, err)
	entries := map[string]string{".tekton/pr.yaml": processed}
	tgitea.PushFilesToRefGit(t, topts, entries, topts.TargetRefName)

	processed, err = payload.ApplyTemplate("testdata/pipelinerun-alt.yaml", map[string]string{
		"TargetNamespace": topts.TargetNS,
		"TargetBranch":    topts.DefaultBranch,
		"TargetEvent":     topts.TargetEvent,
		"PipelineName":    "pr",
		"Command":         "echo SUCCESS",
	})
	assert.NilError(t, err)
	entries = map[string]string{".tekton/pr.yaml": processed}
	tgitea.PushFilesToRefGit(t, topts, entries, topts.TargetRefName)

	// loop until we get the status
	gotPipelineRunPending := false
	for i := 0; i < 30; i++ {
		prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{})
		assert.NilError(t, err)

		// range over prs
		for _, pr := range prs.Items {
			// check for status
			status := pr.Spec.Status
			if status == "PipelineRunPending" {
				gotPipelineRunPending = true
				break
			}
		}
		if gotPipelineRunPending {
			topts.ParamsRun.Clients.Log.Info("Found PipelineRunPending in PipelineRuns")
			break
		}
		time.Sleep(5 * time.Second)
	}
	if !gotPipelineRunPending {
		t.Fatalf("Did not find PipelineRunPending in PipelineRuns")
	}

	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)

	topts.Regexp = successRegexp
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

func TestGiteaRetestAfterPush(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      regexp.MustCompile(`.*has <b>failed</b>`),
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/failures/pipelinerun-exit-1.yaml",
		},
		NoCleanup:      true,
		CheckForStatus: "failure",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()

	newyamlFiles := map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"}
	entries, err := payload.GetEntries(newyamlFiles, topts.TargetNS, topts.DefaultBranch, topts.TargetEvent, map[string]string{})
	assert.NilError(t, err)
	tgitea.PushFilesToRefGit(t, topts, entries, topts.TargetRefName)
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)
}

func TestGiteaConfigMaxKeepRun(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun-max-keep-run-1.yaml",
		},
		NoCleanup:      true,
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()
	tgitea.PostCommentOnPullRequest(t, topts, "/retest")
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(15 * time.Second) // “Evil does not sleep. It waits.” - Galadriel

	prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)

	assert.Equal(t, len(prs.Items), 1, "should have only one pipelinerun, but we have: %d", len(prs.Items))
}

func TestGiteaPush(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request, push",
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
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
	prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{
		LabelSelector: pacapi.EventType + "=push",
	})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1, "should have only one push pipelinerun")
}

func TestGiteaWithCLI(t *testing.T) {
	t.Parallel()
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: "pull_request, push",
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		CheckForStatus: "success",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()
	output, err := tknpactest.ExecCommand(topts.ParamsRun, tknpaclist.Root, "pipelinerun", "list", "-n", topts.TargetNS)
	assert.NilError(t, err)
	match, err := regexp.MatchString(".*(Running|Succeeded)", output)
	assert.NilError(t, err)
	assert.Assert(t, match, "should have a Running or Succeeded pipelinerun in CLI listing: %s", output)

	output, err = tknpactest.ExecCommand(topts.ParamsRun, tknpacdesc.Root, "-n", topts.TargetNS)
	assert.NilError(t, err)
	match, err = regexp.MatchString(".*(Running|Succeeded)", output)
	assert.NilError(t, err)
	assert.Assert(t, match, "should have a Succeeded or Running pipelinerun in CLI describe and auto select the first one: %s", output)

	output, err = tknpactest.ExecCommand(topts.ParamsRun, tknpacdelete.Root, "-n", topts.TargetNS, "repository", topts.TargetNS, "--cascade")
	assert.NilError(t, err)
	expectedOutput := fmt.Sprintf("secret gitea-secret has been deleted\nrepository %s has been deleted\n", topts.TargetNS)
	assert.Assert(t, output == expectedOutput, topts.TargetRefName, "delete command should have this output: %s received: %s", expectedOutput, output)
}

func TestGiteaWithCLIGeneratePipeline(t *testing.T) {
	tests := []struct {
		name                 string
		generateOutputRegexp string
		wantErr              bool
		fileToAdd            map[string]string
	}{
		// we are not testing Java cause pom.xml is weird to get a very simple test
		{
			name: "CLI generate nodejs",
			fileToAdd: map[string]string{
				"package.json": `{
					"name": "whatisthis",
					"version": "1.0.0",
					"description": "",
					"main": "index.js",
					"scripts": {
					  "test": "echo \"Hello Friend\""
					},
					"author": "",
					"license": "BSD"
				  }`,
			},
			generateOutputRegexp: `We have detected your repository using the programming language.*Nodejs`,
		},
		{
			name:                 "CLI generate python",
			generateOutputRegexp: `We have detected your repository using the programming language.*Python`,
			fileToAdd: map[string]string{
				"setup.py":    "# setup.py\n",
				"__init__.py": "# __init__.py\n",
			},
		},
		{
			name:                 "CLI generate golang",
			generateOutputRegexp: `We have detected your repository using the programming language.*Go`,
			fileToAdd: map[string]string{
				"go.mod": "module github.com/mylady/ismybike",
				"main.go": `package main

	import "fmt"

	func main() {
		fmt.Println("Hello World")
	}
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topts := &tgitea.TestOpts{
				Regexp:      successRegexp,
				TargetEvent: "pull_request, push",
				YAMLFiles: map[string]string{
					".tekton/pr.yaml": "testdata/pipelinerun.yaml",
				},
				CheckForStatus: "success",
				ExpectEvents:   false,
			}
			defer tgitea.TestPR(t, topts)()
			tmpdir, dirCleanups := tgitea.InitGitRepo(t)
			defer dirCleanups()
			_, err := git.RunGit(tmpdir, "remote", "add", "-t", topts.TargetNS, "-f", "origin", topts.GitCloneURL)
			assert.NilError(t, err)
			_, err = git.RunGit(tmpdir, "checkout", "-B", topts.TargetNS, "origin/"+topts.TargetNS)
			assert.NilError(t, err)

			for k, v := range tt.fileToAdd {
				newFile, err2 := os.Create(filepath.Join(tmpdir, k))
				assert.NilError(t, err2)
				_, err2 = newFile.WriteString(v)
				assert.NilError(t, err2)
				_, err2 = git.RunGit(tmpdir, "add", k)
				assert.NilError(t, err2)
			}

			output, err := tknpactest.ExecCommand(topts.ParamsRun, tknpacgenerate.Command, "--event-type", topts.TargetEvent,
				"--branch", topts.DefaultBranch, "--file-name", ".tekton/pr.yaml", "--overwrite")
			assert.NilError(t, err)
			assert.Assert(t, regexp.MustCompile(tt.generateOutputRegexp).MatchString(output))

			envRemove := env.PatchAll(t, map[string]string{"PAC_PROVIDER_TOKEN": "NOWORRIESBEHAPPY"})
			defer envRemove()
			topts.ParamsRun.Info.Pac = &info.PacOpts{}
			topts.ParamsRun.Info.Pac.Settings = &settings.Settings{}
			_, err = tknpactest.ExecCommand(topts.ParamsRun, tknpacresolve.Command, "-f", ".tekton/pr.yaml", "-p", "revision=main")
			assert.NilError(t, err)

			// edit .tekton/pr.yaml file
			pryaml, err := os.ReadFile(filepath.Join(tmpdir, ".tekton/pr.yaml"))
			// replace with regexp
			reg := regexp.MustCompile(`.*- name: url\n.*`)
			// we need this for gitea to work so we do what we have to do and life goes on until
			b := reg.ReplaceAllString(string(pryaml),
				fmt.Sprintf("          - name: url\n            value: %s/%s\n          - name: sslVerify\n            value: false",
					topts.InternalGiteaURL,
					topts.PullRequest.Base.Repository.FullName))
			assert.NilError(t, err)
			err = os.WriteFile(filepath.Join(tmpdir, ".tekton/pr.yaml"), []byte(b), 0o600)
			assert.NilError(t, err)

			_, err = git.RunGit(tmpdir, "add", ".tekton/pr.yaml")
			assert.NilError(t, err)

			_, err = git.RunGit(tmpdir, "commit", "-a", "-m", "it's a beautiful day")
			assert.NilError(t, err)

			_, err = git.RunGit(tmpdir, "push", "origin", topts.TargetRefName)
			assert.NilError(t, err)

			tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)

			prs, err := topts.ParamsRun.Clients.Tekton.TektonV1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{
				LabelSelector: pacapi.EventType + "=pull_request",
			})
			assert.NilError(t, err)
			assert.Assert(t, len(prs.Items) >= 1, "should have at least 1 pipelineruns")
		})
	}
}

func TestGiteaCancelRun(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun_long_running.yaml",
		},
		ExpectEvents: false,
	}
	defer tgitea.TestPR(t, topts)()

	// let pipelineRun start and then cancel it
	time.Sleep(time.Second * 2)
	tgitea.PostCommentOnPullRequest(t, topts, "/cancel")

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err := twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.Error(t, err, "pipelinerun has failed")

	tgitea.CheckIfPipelineRunsCancelled(t, topts)
}

func TestGiteaConcurrencyOrderedExecution(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      successRegexp,
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelineruns-ordered-execution.yaml",
		},
		CheckForStatus:       "success",
		CheckForNumberStatus: 3,
		ConcurrencyLimit:     github.Int(1),
		ExpectEvents:         false,
	}
	defer tgitea.TestPR(t, topts)()

	repo, err := topts.ParamsRun.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Get(context.Background(), topts.TargetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	// check the last 3 update in RepositoryRunStatus are in order
	statusLen := len(repo.Status)
	assert.Assert(t, strings.HasPrefix(repo.Status[statusLen-3].PipelineRunName, "abc"))
	assert.Assert(t, strings.HasPrefix(repo.Status[statusLen-2].PipelineRunName, "pqr"))
	assert.Assert(t, strings.HasPrefix(repo.Status[statusLen-1].PipelineRunName, "xyz"))
	time.Sleep(time.Second * 10)
}

func TestGiteaErrorSnippet(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun-error-snippet.yaml",
		},
		CheckForStatus: "failure",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()

	topts.Regexp = regexp.MustCompile(`Hey man i just wanna to say i am not such a failure, i am useful in my failure`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

func TestGiteaErrorSnippetWithSecret(t *testing.T) {
	var err error
	ctx := context.Background()
	topts := &tgitea.TestOpts{
		TargetRefName: names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"),
	}
	topts.TargetNS = topts.TargetRefName
	topts.ParamsRun, topts.Opts, topts.GiteaCNX, err = tgitea.Setup(ctx)
	assert.NilError(t, err, fmt.Errorf("cannot do gitea setup: %w", err))
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun))
	assert.NilError(t, secret.Create(ctx, topts.ParamsRun, map[string]string{"secret": "SHHHHHHH"}, topts.TargetNS, "pac-secret"))
	topts.TargetEvent = options.PullRequestEvent
	topts.YAMLFiles = map[string]string{
		".tekton/pr.yaml": "testdata/pipelinerun-error-snippet-with-secret.yaml",
	}
	topts.CheckForStatus = "failure"
	defer tgitea.TestPR(t, topts)()

	topts.Regexp = regexp.MustCompile(`I WANT TO SAY \*\*\*\*\* OUT LOUD BUT NOBODY UNDERSTAND ME`)
	tgitea.WaitForPullRequestCommentMatch(t, topts)
}

// TestGiteaNotExistingClusterTask checks that the pipeline run fails if the clustertask does not exist
// This willl test properly if we error the reason in UI see bug #1160
func TestGiteaNotExistingClusterTask(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      regexp.MustCompile(`.*clustertasks.tekton.dev "foo-bar" not found`),
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/failures/not-existing-clustertask.yaml",
		},
		NoCleanup:      true,
		CheckForStatus: "failure",
		ExpectEvents:   false,
	}
	defer tgitea.TestPR(t, topts)()
}

// TestGiteaBadLinkOfTask checks that we fail properly with the error from the
// tekton pipelines controlller. We check on the UI interface that we display
// and inside the pac controller.
func TestGiteaBadLinkOfTask(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/failures/bad-runafter-task.yaml",
		},
		NoCleanup:      true,
		CheckForStatus: "failure",
		ExpectEvents:   true,
		Regexp:         regexp.MustCompile(".*There was an error creating the PipelineRun*"),
	}
	defer tgitea.TestPR(t, topts)()
	errre := regexp.MustCompile("There was an error starting the PipelineRun")
	assert.NilError(t, twait.RegexpMatchingInControllerLog(context.Background(), topts.ParamsRun, *errre, 10))
}

// TestGiteaParamsOnRepoCR test gitea params on CR and its filters
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

	// Checking repo status length to avoid index out of range [0] with length 0 issue
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

func TestGiteaParamsOnRepoCRWithCustomConsole(t *testing.T) {
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

func TestGiteaProvenance(t *testing.T) {
	topts := &tgitea.TestOpts{
		SkipEventsCheck:       true,
		TargetEvent:           options.PullRequestEvent,
		Settings:              &v1alpha1.Settings{PipelineRunProvenance: "default_branch"},
		NoPullRequestCreation: true,
	}
	defer tgitea.TestPR(t, topts)()
	branch := topts.TargetRefName
	prmap := map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"}
	entries, err := payload.GetEntries(prmap, topts.TargetNS, topts.DefaultBranch, topts.TargetEvent, map[string]string{})
	assert.NilError(t, err)
	topts.TargetRefName = topts.DefaultBranch
	tgitea.PushFilesToRefGit(t, topts, entries, topts.DefaultBranch)
	topts.TargetRefName = branch
	prmap = map[string]string{"notgonnatobetested.yaml": "testdata/pipelinerun.yaml"}
	entries, err = payload.GetEntries(prmap, topts.TargetNS, topts.DefaultBranch, topts.TargetEvent, map[string]string{})
	assert.NilError(t, err)
	tgitea.PushFilesToRefGit(t, topts, entries, topts.DefaultBranch)

	pr, _, err := topts.GiteaCNX.Client.CreatePullRequest(topts.Opts.Organization, topts.TargetRefName, gitea.CreatePullRequestOption{
		Title: "Test Pull Request - " + topts.TargetRefName,
		Head:  topts.TargetRefName,
		Base:  options.MainBranch,
	})
	assert.NilError(t, err)
	topts.PullRequest = pr
	topts.ParamsRun.Clients.Log.Infof("PullRequest %s has been created", pr.HTMLURL)
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, "heads/"+topts.TargetRefName, "", false)
}

func TestGiteaPushToTagGreedy(t *testing.T) {
	topts := &tgitea.TestOpts{
		SkipEventsCheck:       true,
		TargetEvent:           options.PushEvent,
		NoPullRequestCreation: true,
	}
	defer tgitea.TestPR(t, topts)()
	prmap := map[string]string{".tekton/pr.yaml": "testdata/pipelinerun.yaml"}
	entries, err := payload.GetEntries(prmap, topts.TargetNS, "refs/tags/*", topts.TargetEvent, map[string]string{})
	assert.NilError(t, err)
	topts.TargetRefName = topts.DefaultBranch
	tgitea.PushFilesToRefGit(t, topts, entries, topts.DefaultBranch)

	topts.TargetRefName = "refs/tags/v1.0.0"
	tgitea.PushFilesToRefGit(t, topts, map[string]string{"README.md": "hello new version from tag"}, topts.DefaultBranch)

	waitOpts := twait.Opts{
		RepoName:  topts.TargetNS,
		Namespace: topts.TargetNS,
		// 0 means 1 🙃 (we test for >, while we actually should do >=, but i
		// need to go all over the code to verify it's not going to break
		// anything else)
		MinNumberStatus: 0,
		PollTimeout:     twait.DefaultTimeout,
	}
	err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)
}

// TestGiteaClusterTasks is a test to verify that we can use cluster tasks with PaaC
func TestGiteaClusterTasks(t *testing.T) {
	// we need to verify sure to create clustertask before pushing the files
	// so we have to create a new client and do more manual things we get for free in TestPR
	topts := &tgitea.TestOpts{
		TargetEvent: "pull_request, push",
		YAMLFiles: map[string]string{
			".tekton/prcluster.yaml": "testdata/pipelinerunclustertasks.yaml",
		},
		ExpectEvents: false,
	}
	topts.TargetRefName = names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test")
	topts.TargetNS = topts.TargetRefName

	// create first the cluster tasks
	ctname := fmt.Sprintf(".tekton/%s.yaml", topts.TargetNS)
	newyamlFiles := map[string]string{ctname: "testdata/clustertask.yaml"}
	entries, err := payload.GetEntries(newyamlFiles, topts.TargetNS, "main", "pull_request", map[string]string{})
	assert.NilError(t, err)
	//nolint: staticcheck
	ct := v1beta1.ClusterTask{}
	assert.NilError(t, yaml.Unmarshal([]byte(entries[ctname]), &ct))
	ct.Name = "clustertask-" + topts.TargetNS

	run := &params.Run{}
	ctx := context.Background()
	assert.NilError(t, run.Clients.NewClients(ctx, &run.Info))
	// TODO(chmou): this is for v1beta1, we need to figure out a way how to do that on v1
	_, err = run.Clients.Tekton.TektonV1beta1().ClusterTasks().Create(context.TODO(), &ct, metav1.CreateOptions{})
	assert.NilError(t, err)
	assert.NilError(t, pacrepo.CreateNS(ctx, topts.TargetNS, run))
	run.Clients.Log.Infof("%s has been created", ct.GetName())
	defer (func() {
		assert.NilError(t, topts.ParamsRun.Clients.Tekton.TektonV1beta1().ClusterTasks().Delete(context.TODO(), ct.Name, metav1.DeleteOptions{}))
		run.Clients.Log.Infof("%s is deleted", ct.GetName())
	})()

	// start PR
	defer tgitea.TestPR(t, topts)()

	// wait for it
	waitOpts := twait.Opts{
		RepoName:  topts.TargetNS,
		Namespace: topts.TargetNS,
		// 0 means 1 🙃 (we test for >, while we actually should do >=, but i
		// need to go all over the code to verify it's not going to break
		// anything else)
		MinNumberStatus: 0,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err = twait.UntilRepositoryUpdated(context.Background(), topts.ParamsRun.Clients, waitOpts)
	assert.NilError(t, err)

	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, topts.TargetRefName, "", true)
}

func TestGiteaStandardParamsCheckForPushAndPullEvent(t *testing.T) {
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

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGiteaPush ."
// End:
