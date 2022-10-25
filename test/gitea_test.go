//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v47/github"
	pacapi "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	tknpacdelete "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/deleterepo"
	tknpacdesc "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/describe"
	tknpacgenerate "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/generate"
	tknpaclist "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/list"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tknpactest "github.com/openshift-pipelines/pipelines-as-code/test/pkg/cli"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/names"
	"gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
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

	assert.NilError(t, twait.RegexpMatchingInPodLog(ctx, topts.Clients, "app.kubernetes.io/component=controller", "pac-controller", *regexp.MustCompile(
		fmt.Sprintf("PipelineRun pr-bad-format-%s- has failed:.*validation failed", topts.TargetNS)),
		10))
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

	time.Sleep(5 * time.Second)

	prs, err := topts.Clients.Clients.Tekton.TektonV1beta1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)

	// range over prs
	gotPipelineRunPending := false
	for _, pr := range prs.Items {
		// check for status
		status := pr.Spec.Status
		if status == "PipelineRunPending" {
			gotPipelineRunPending = true
		}
	}
	if !gotPipelineRunPending {
		t.Fatalf("Expected to get a PipelineRunPending status in one of the PR but we didn't, maybe a race but that would be very unlucky")
	} else {
		topts.Clients.Clients.Log.Info("Found PipelineRunPending in PipelineRuns")
	}
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)

	topts.Regexp = successRegexp
	tgitea.WaitForPullRequestCommentMatch(context.Background(), t, topts)
}

func TestGiteaRetestAfterPush(t *testing.T) {
	topts := &tgitea.TestOpts{
		Regexp:      regexp.MustCompile(`.*pr has.*failed`),
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
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)
}

func TestGiteaACLOrgAllowed(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		NoCleanup:    true,
		ExpectEvents: false,
	}
	defer tgitea.TestPR(t, topts)()
	secondcnx, err := tgitea.CreateGiteaUser(topts.GiteaCNX.Client, topts.GiteaAPIURL, topts.TargetRefName, topts.GiteaPassword)
	assert.NilError(t, err)

	tgitea.CreateForkPullRequest(t, topts, secondcnx, "read", "echo Hello from user "+topts.TargetRefName)
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)
}

func TestGiteaACLOrgSkipped(t *testing.T) {
	topts := &tgitea.TestOpts{
		TargetEvent: options.PullRequestEvent,
		YAMLFiles: map[string]string{
			".tekton/pr.yaml": "testdata/pipelinerun.yaml",
		},
		NoCleanup:    true,
		ExpectEvents: false,
	}
	defer tgitea.TestPR(t, topts)()
	secondcnx, err := tgitea.CreateGiteaUser(topts.GiteaCNX.Client, topts.GiteaAPIURL, topts.TargetRefName, topts.GiteaPassword)
	assert.NilError(t, err)

	topts.PullRequest = tgitea.CreateForkPullRequest(t, topts, secondcnx, "", "echo Hello from user "+topts.TargetRefName)
	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha)
	topts.Regexp = regexp.MustCompile(`.*is skipping this commit.*is not allowed.*`)
	tgitea.WaitForPullRequestCommentMatch(context.Background(), t, topts)
}

func TestGiteaACLCommentsAllowing(t *testing.T) {
	tests := []struct {
		name, comment string
	}{
		{
			name:    "OK to Test",
			comment: "/ok-to-test",
		},
		{
			name:    "Retest",
			comment: "/retest",
		},
		{
			name:    "Test PR",
			comment: "/test pr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topts := &tgitea.TestOpts{
				TargetEvent: options.PullRequestEvent,
				YAMLFiles: map[string]string{
					".tekton/pr.yaml": "testdata/pipelinerun.yaml",
				},
				NoCleanup:    true,
				ExpectEvents: false,
			}
			defer tgitea.TestPR(t, topts)()
			secondcnx, err := tgitea.CreateGiteaUser(topts.GiteaCNX.Client, topts.GiteaAPIURL, topts.TargetRefName, topts.GiteaPassword)
			assert.NilError(t, err)

			topts.PullRequest = tgitea.CreateForkPullRequest(t, topts, secondcnx, "", "echo Hello from user "+topts.TargetRefName)
			topts.CheckForStatus = "success"
			tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha)
			topts.Regexp = regexp.MustCompile(`.*is skipping this commit.*is not allowed.*`)
			tgitea.WaitForPullRequestCommentMatch(context.Background(), t, topts)

			tgitea.PostCommentOnPullRequest(t, topts, tt.comment)
			topts.Regexp = successRegexp
			tgitea.WaitForPullRequestCommentMatch(context.Background(), t, topts)
		})
	}
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
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)

	waitOpts := twait.Opts{
		RepoName:        topts.TargetNS,
		Namespace:       topts.TargetNS,
		MinNumberStatus: 1, // 1 means 2 🙃
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err := twait.UntilRepositoryUpdated(context.Background(), topts.Clients.Clients, waitOpts)
	assert.NilError(t, err)

	time.Sleep(15 * time.Second) // “Evil does not sleep. It waits.” - Galadriel

	prs, err := topts.Clients.Clients.Tekton.TektonV1beta1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{})
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
	tgitea.WaitForStatus(t, topts, topts.PullRequest.Head.Sha)
	time.Sleep(5 * time.Second)
	prs, err := topts.Clients.Clients.Tekton.TektonV1beta1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{
		LabelSelector: filepath.Join(pacapi.GroupName, "event-type") + "=push",
	})
	assert.NilError(t, err)
	assert.Equal(t, len(prs.Items), 1, "should have only one push pipelinerun")
}

func TestGiteaClusterTasks(t *testing.T) {
	// we need to make sure to create clustertask before pushing the files
	// so we have to create a new client and do a lot of manual things we get for free in TestPR
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
	ct := v1beta1.ClusterTask{}
	assert.NilError(t, yaml.Unmarshal([]byte(entries[ctname]), &ct))
	ct.ObjectMeta.Name = "clustertask-" + topts.TargetNS

	run := &params.Run{}
	assert.NilError(t, run.Clients.NewClients(context.Background(), &run.Info))
	_, err = run.Clients.Tekton.TektonV1beta1().ClusterTasks().Create(context.TODO(), &ct, metav1.CreateOptions{})
	assert.NilError(t, err)
	run.Clients.Log.Infof("%s has been created", ct.GetName())
	defer (func() {
		assert.NilError(t, topts.Clients.Clients.Tekton.TektonV1beta1().ClusterTasks().Delete(context.TODO(), ct.ObjectMeta.Name, metav1.DeleteOptions{}))
		run.Clients.Log.Infof("%s is deleted", ct.GetName())
	})()

	// start PR
	defer tgitea.TestPR(t, topts)()

	// wait for it
	waitOpts := twait.Opts{
		RepoName:  topts.TargetNS,
		Namespace: topts.TargetNS,
		// 0 means 1 🙃 (we test for >, while we actually should do >=, but i
		// need to go all over the code to make sure it's not going to break
		// anything else)
		MinNumberStatus: 0,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       topts.PullRequest.Head.Sha,
	}
	err = twait.UntilRepositoryUpdated(context.Background(), topts.Clients.Clients, waitOpts)
	assert.NilError(t, err)

	topts.CheckForStatus = "success"
	tgitea.WaitForStatus(t, topts, topts.TargetRefName)
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
	output, err := tknpactest.ExecCommand(topts.Clients, tknpaclist.Root, "pipelinerun", "list", "-n", topts.TargetNS)
	assert.NilError(t, err)
	match, err := regexp.MatchString(".*(Running|Succeeded)", output)
	assert.NilError(t, err)
	assert.Assert(t, match, "should have a Running or Succeeded pipelinerun in CLI listing: %s", output)

	output, err = tknpactest.ExecCommand(topts.Clients, tknpacdesc.Root, "-n", topts.TargetNS)
	assert.NilError(t, err)
	match, err = regexp.MatchString(".*(Running|Succeeded)", output)
	assert.NilError(t, err)
	assert.Assert(t, match, "should have a Succeeded or Running pipelinerun in CLI describe and auto select the first one: %s", output)

	output, err = tknpactest.ExecCommand(topts.Clients, tknpacdelete.Root, "-n", topts.TargetNS, "repository", topts.TargetNS, "--cascade")
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
				newFile, err := os.Create(filepath.Join(tmpdir, k))
				assert.NilError(t, err)
				_, err = newFile.WriteString(v)
				assert.NilError(t, err)
				defer newFile.Close()
				_, err = git.RunGit(tmpdir, "add", k)
				assert.NilError(t, err)
			}

			output, err := tknpactest.ExecCommand(topts.Clients, tknpacgenerate.Command, "--event-type", topts.TargetEvent,
				"--branch", topts.DefaultBranch, "--file-name", ".tekton/pr.yaml", "--overwrite")
			assert.NilError(t, err)
			assert.Assert(t, regexp.MustCompile(tt.generateOutputRegexp).MatchString(output))

			// edit .tekton/pr.yaml file
			pryaml, err := os.ReadFile(filepath.Join(tmpdir, ".tekton/pr.yaml"))
			// replace with regexp
			reg := regexp.MustCompile(`.*- name: url\n.*`)
			// we need this for gitea to work so we do what we have to do and life goes on until
			b := reg.ReplaceAllString(string(pryaml), fmt.Sprintf("          - name: url\n            value: http://gitea.gitea:3000/%s\n          - name: sslVerify\n            value: false",
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

			tgitea.WaitForStatus(t, topts, topts.TargetRefName)

			prs, err := topts.Clients.Clients.Tekton.TektonV1beta1().PipelineRuns(topts.TargetNS).List(context.Background(), metav1.ListOptions{
				LabelSelector: filepath.Join(pacapi.GroupName, "event-type") + "=pull_request",
			})
			assert.NilError(t, err)
			assert.Assert(t, len(prs.Items) >= 2, "should have at least 2 pipelineruns")
		})
	}
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestGiteaPush ."
// End:
