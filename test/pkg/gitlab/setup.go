package gitlab

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/setup"
	gitlab2 "gitlab.com/gitlab-org/api/client-go"
	"gotest.tools/v3/assert"
)

func Setup(ctx context.Context) (*params.Run, options.E2E, gitlab.Provider, error) {
	if err := setup.RequireEnvs(
		"TEST_GITLAB_API_URL",
		"TEST_GITLAB_TOKEN",
		"TEST_GITLAB_PROJECT_ID",
		"TEST_EL_WEBHOOK_SECRET",
		"TEST_EL_URL",
	); err != nil {
		return nil, options.E2E{}, gitlab.Provider{}, err
	}
	gitlabURL := os.Getenv("TEST_GITLAB_API_URL")
	gitlabToken := os.Getenv("TEST_GITLAB_TOKEN")
	sgitlabPid := os.Getenv("TEST_GITLAB_PROJECT_ID")
	gitlabPid, err := strconv.Atoi(sgitlabPid)
	if err != nil {
		return nil, options.E2E{}, gitlab.Provider{}, fmt.Errorf("TEST_GITLAB_PROJECT_ID env variable must be an integer, found '%v': %w", gitlabPid, err)
	}

	controllerURL := os.Getenv("TEST_EL_URL")

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, gitlab.Provider{}, err
	}

	e2eoptions := options.E2E{
		ProjectID:     gitlabPid,
		ControllerURL: controllerURL,
		UserName:      "git",
		Password:      gitlabToken,
	}
	glprovider := gitlab.Provider{}
	err = glprovider.SetClient(ctx, run, &info.Event{
		Provider: &info.Provider{
			Token: gitlabToken,
			URL:   gitlabURL,
		},
	}, nil, nil)
	if err != nil {
		return nil, options.E2E{}, gitlab.Provider{}, err
	}
	return run, e2eoptions, glprovider, nil
}

func TearDown(ctx context.Context, t *testing.T, runcnx *params.Run, glprovider gitlab.Provider, mrNumber int, ref, targetNS string, projectid int) {
	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up and closing PR since TEST_NOCLEANUP is set")
		return
	}

	if mrNumber != -1 {
		runcnx.Clients.Log.Infof("Closing PR %d", mrNumber)
		_, _, err := glprovider.Client().MergeRequests.UpdateMergeRequest(projectid, int64(mrNumber),
			&gitlab2.UpdateMergeRequestOptions{StateEvent: gitlab2.Ptr("close")})
		if err != nil {
			t.Fatal(err)
		}
	}
	repository.NSTearDown(ctx, t, runcnx, targetNS)
	if ref != "" {
		runcnx.Clients.Log.Infof("Deleting Ref %s", ref)
		_, err := glprovider.Client().Branches.DeleteBranch(projectid, ref)
		assert.NilError(t, err)
	}
}

// CleanTag a wrapper function on DeleteTag to ignore errors in cleaning.
func CleanTag(client *gitlab2.Client, pid int, tagName string) {
	_ = DeleteTag(client, pid, tagName)
}
