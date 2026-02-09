package wait

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var DefaultTimeout = 10 * time.Minute

type SuccessOpt struct {
	TargetNS        string
	OnEvent         string
	SHA             string
	Title           string
	MinNumberStatus int

	NumberofPRMatch int
}

func Succeeded(ctx context.Context, t *testing.T, runcnx *params.Run, opts options.E2E, sopt SuccessOpt) {
	runcnx.Clients.Log.Infof("Waiting for Repository to be updated")
	minNumberStatus := sopt.MinNumberStatus
	if minNumberStatus == 0 {
		minNumberStatus = sopt.NumberofPRMatch
	}
	waitOpts := Opts{
		RepoName:        sopt.TargetNS,
		Namespace:       sopt.TargetNS,
		MinNumberStatus: minNumberStatus,
		PollTimeout:     DefaultTimeout,
		TargetSHA:       sopt.SHA,
	}
	_, err := UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(sopt.TargetNS).Get(ctx, sopt.TargetNS, v1.GetOptions{})
	assert.NilError(t, err)

	laststatus := repo.Status[len(repo.Status)-1]
	if sopt.SHA != "" {
		for i := len(repo.Status) - 1; i >= 0; i-- {
			if repo.Status[i].SHA != nil && *repo.Status[i].SHA == sopt.SHA {
				laststatus = repo.Status[i]
				break
			}
		}
	}
	assert.Equal(t, corev1.ConditionTrue, laststatus.Conditions[0].Status)
	if sopt.SHA != "" {
		assert.Equal(t, sopt.SHA, *laststatus.SHA)
		assert.Equal(t, sopt.SHA, filepath.Base(*laststatus.SHAURL))
	}
	laststatustitle := strings.TrimSpace(*laststatus.Title)
	if sopt.Title != "" {
		assert.Equal(t, sopt.Title, laststatustitle)
	} else {
		assert.Assert(t, *laststatus.Title != "")
	}
	assert.Assert(t, *laststatus.LogURL != "")

	pr, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(sopt.TargetNS).Get(ctx, laststatus.PipelineRunName, v1.GetOptions{})
	assert.NilError(t, err)

	assert.Equal(t, sopt.OnEvent, pr.Annotations[keys.EventType])
	assert.Equal(t, repo.GetName(), pr.Annotations[keys.Repository])

	if opts.Organization != "" {
		assert.Equal(t, opts.Organization, pr.Annotations[keys.URLOrg])
	}
	if opts.Repo != "" {
		assert.Equal(t, opts.Repo, pr.Annotations[keys.URLRepository])
	}
	if sopt.SHA != "" {
		assert.Equal(t, sopt.SHA, pr.Annotations[keys.SHA])
		assert.Equal(t, sopt.SHA, filepath.Base(pr.Annotations[keys.ShaURL]))
	}
	assert.Equal(t, laststatustitle, strings.TrimSpace(pr.Annotations[keys.ShaTitle]))
	runcnx.Clients.Log.Infof("Success, number of status %d has been matched", sopt.NumberofPRMatch)
}
