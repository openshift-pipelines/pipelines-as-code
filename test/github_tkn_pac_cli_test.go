//go:build e2e
// +build e2e

package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	tkpacrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/repository"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/spf13/cobra"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func execCommand(runcnx *params.Run, cmd func(*params.Run, *cli.IOStreams) *cobra.Command,
	args ...string) (string, error) {
	bufout := new(bytes.Buffer)
	ecmd := cmd(runcnx, &cli.IOStreams{
		Out: bufout,
	})
	_, err := tcli.ExecuteCommand(ecmd, args...)
	return bufout.String(), err
}

func TestGithubPacCli(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	ctx := context.Background()
	runcnx, opts, ghprovider, err := githubSetup(ctx, false)
	assert.NilError(t, err)

	entries := map[string]string{
		".tekton/info.yaml": fmt.Sprintf(`---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipeline
  annotations:
    pipelinesascode.tekton.dev/target-namespace: "%s"
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"
    pipelinesascode.tekton.dev/on-event: "[%s]"
spec:
  pipelineSpec:
    tasks:
      - name: task
        taskSpec:
          steps:
            - name: task
              image: gcr.io/google-containers/busybox
              command: ["/bin/echo", "HELLOMOTO"]
`, targetNS, mainBranch, pullRequestEvent),
	}

	repoinfo, resp, err := ghprovider.Client.Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}

	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			URL: repoinfo.GetHTMLURL(),
		},
	}

	err = trepo.CreateNS(ctx, targetNS, runcnx)
	assert.NilError(t, err)

	err = trepo.CreateRepo(ctx, targetNS, runcnx, repository)
	assert.NilError(t, err)

	output, err := execCommand(runcnx, tkpacrepo.DescribeCommand, "-n", targetNS, targetNS)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "No runs has started."))

	output, err = execCommand(runcnx, tkpacrepo.ListCommand, "-n", targetNS)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "NoRun"))

	targetRefName := fmt.Sprintf("refs/heads/%s",
		names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-test"))

	sha, err := tgithub.PushFilesToRef(ctx, ghprovider.Client, "TestPacCli - "+targetRefName, repoinfo.GetDefaultBranch(), targetRefName, opts.Organization, opts.Repo, entries)
	assert.NilError(t, err)
	runcnx.Clients.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)

	title := "TestPacCli - " + targetRefName
	number, err := tgithub.PRCreate(ctx, runcnx, ghprovider, opts.Organization, opts.Repo, targetRefName, repoinfo.GetDefaultBranch(), title)
	assert.NilError(t, err)

	defer ghtearDown(ctx, t, runcnx, ghprovider, number, targetRefName, targetNS, opts)

	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")
	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: 0,
		PollTimeout:     defaultTimeout,
		TargetSHA:       sha,
	}
	err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")

	output, err = execCommand(runcnx, tkpacrepo.ListCommand, "-n", targetNS)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "Succeeded"))

	output, err = execCommand(runcnx, tkpacrepo.DescribeCommand, "-n", targetNS, targetNS)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "Succeeded"))
}

// Local Variables:
// compile-command: "go test -tags=e2e -v -run TestPacCli$ ."
// End:
