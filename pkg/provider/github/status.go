package github

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/go-github/v45/github"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const taskStatusTemplate = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>

{{- range $taskrun := .TaskRunList }}
<tr>
<td>{{ formatCondition $taskrun.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`

const checkRunIDKey = "check-run-id"

func getCheckName(status provider.StatusOpts, pacopts *info.PacOpts) string {
	if pacopts.ApplicationName != "" {
		if status.OriginalPipelineRunName == "" {
			return pacopts.ApplicationName
		}
		return fmt.Sprintf("%s / %s", pacopts.ApplicationName, status.OriginalPipelineRunName)
	}
	return status.OriginalPipelineRunName
}

func (v *Provider) getExistingCheckRunID(ctx context.Context, runevent *info.Event, status provider.StatusOpts) (*int64, error) {
	res, _, err := v.Client.Checks.ListCheckRunsForRef(ctx, runevent.Organization, runevent.Repository,
		runevent.SHA, &github.ListCheckRunsOptions{
			AppID: v.ApplicationID,
		})
	if err != nil {
		return nil, err
	}
	if *res.Total == 0 {
		return nil, nil
	}

	for _, checkrun := range res.CheckRuns {
		if *checkrun.ExternalID == status.PipelineRunName {
			return checkrun.ID, nil
		}
	}

	return nil, nil
}

func (v *Provider) createCheckRunStatus(ctx context.Context, runevent *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) (*int64, error) {
	now := github.Timestamp{Time: time.Now()}
	checkrunoption := github.CreateCheckRunOptions{
		Name:       getCheckName(status, pacopts),
		HeadSHA:    runevent.SHA,
		Status:     github.String("in_progress"),
		DetailsURL: github.String(pacopts.LogURL),
		ExternalID: github.String(status.PipelineRunName),
		StartedAt:  &now,
	}

	checkRun, _, err := v.Client.Checks.CreateCheckRun(ctx, runevent.Organization, runevent.Repository, checkrunoption)
	if err != nil {
		return nil, err
	}
	return checkRun.ID, nil
}

// getOrUpdateCheckRunStatus create a status via the checkRun API, which is only
// available with Github apps tokens.
func (v *Provider) getOrUpdateCheckRunStatus(ctx context.Context, tekton versioned.Interface, runevent *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) error {
	var err error
	var checkRunID *int64
	var found bool

	// check if pipelineRun has the label with checkRun-id
	if status.PipelineRun != nil {
		var id string
		id, found = status.PipelineRun.GetLabels()[filepath.Join(apipac.GroupName, checkRunIDKey)]
		if found {
			checkID, err := strconv.Atoi(id)
			if err != nil {
				return fmt.Errorf("api error: cannot convert checkrunid")
			}
			checkRunID = github.Int64(int64(checkID))
		}
	}

	if !found {
		if checkRunID, _ = v.getExistingCheckRunID(ctx, runevent, status); checkRunID == nil {
			checkRunID, err = v.createCheckRunStatus(ctx, runevent, pacopts, status)
			if err != nil {
				return err
			}
		}
		if err := v.updatePipelineRunWithCheckRunID(ctx, tekton, status.PipelineRun, checkRunID); err != nil {
			return err
		}
	}

	checkRunOutput := &github.CheckRunOutput{
		Title:   &status.Title,
		Summary: &status.Summary,
		Text:    &status.Text,
	}

	opts := github.UpdateCheckRunOptions{
		Name:   getCheckName(status, pacopts),
		Status: &status.Status,
		Output: checkRunOutput,
	}

	if status.DetailsURL != "" {
		opts.DetailsURL = &status.DetailsURL
	}

	// Only set completed-at if conclusion is set (which means finished)
	if status.Conclusion != "" && status.Conclusion != "pending" {
		opts.CompletedAt = &github.Timestamp{Time: time.Now()}
		opts.Conclusion = &status.Conclusion
	}

	_, _, err = v.Client.Checks.UpdateCheckRun(ctx, runevent.Organization, runevent.Repository, *checkRunID, opts)
	return err
}

func (v *Provider) updatePipelineRunWithCheckRunID(ctx context.Context, tekton versioned.Interface, pr *v1beta1.PipelineRun, checkRunID *int64) error {
	if pr == nil {
		return nil
	}
	maxRun := 10
	for i := 0; i < maxRun; i++ {
		mergePatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					filepath.Join(apipac.GroupName, checkRunIDKey): strconv.FormatInt(*checkRunID, 10),
				},
			},
		}
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return err
		}

		updatedPR, err := tekton.TektonV1beta1().PipelineRuns(pr.Namespace).Patch(ctx, pr.GetName(), types.MergePatchType, patch, v1.PatchOptions{})
		if err != nil {
			v.Logger.Infof("Could not patch Pipelinerun with checkRunID, retrying %v/%v: %v", pr.GetNamespace(), pr.GetName(), err)
			continue
		}

		v.Logger.Infof("PipelineRun %v/%v patched with checkRunID : %v", pr.GetNamespace(), pr.GetName(), updatedPR.Labels[filepath.Join(apipac.GroupName, checkRunIDKey)])
		return nil
	}
	return fmt.Errorf("cannot patch pipelineRun %v/%v with checkRunID", pr.GetNamespace(), pr.GetName())
}

// createStatusCommit use the classic/old statuses API which is available when we
// don't have a github app token
func (v *Provider) createStatusCommit(ctx context.Context, runevent *info.Event, pacopts *info.PacOpts, status provider.StatusOpts) error {
	var err error
	now := time.Now()
	switch status.Conclusion {
	case "skipped":
		status.Conclusion = "success" // We don't have a choice than setting as success, no pending here.
	case "neutral":
		status.Conclusion = "success" // We don't have a choice than setting as success, no pending here.
	}
	if status.Status == "in_progress" {
		status.Conclusion = "pending"
	}

	ghstatus := &github.RepoStatus{
		State:       github.String(status.Conclusion),
		TargetURL:   github.String(status.DetailsURL),
		Description: github.String(status.Title),
		Context:     github.String(getCheckName(status, pacopts)),
		CreatedAt:   &now,
	}

	if _, _, err := v.Client.Repositories.CreateStatus(ctx,
		runevent.Organization, runevent.Repository, runevent.SHA, ghstatus); err != nil {
		return err
	}
	if status.Status == "completed" && status.Text != "" && runevent.EventType == "pull_request" {
		_, _, err = v.Client.Issues.CreateComment(ctx, runevent.Organization, runevent.Repository,
			runevent.PullRequestNumber,
			&github.IssueComment{
				Body: github.String(fmt.Sprintf("%s<br>%s", status.Summary, status.Text)),
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Provider) CreateStatus(ctx context.Context, tekton versioned.Interface, runevent *info.Event, pacopts *info.PacOpts, statusOpts provider.StatusOpts) error {
	if v.Client == nil {
		return fmt.Errorf("cannot set status on github no token or url set")
	}

	switch statusOpts.Conclusion {
	case "success":
		statusOpts.Title = "Success"
		statusOpts.Summary = "has <b>successfully</b> validated your commit."
	case "failure":
		statusOpts.Title = "Failed"
		statusOpts.Summary = "has <b>failed</b>."
	case "skipped":
		statusOpts.Title = "Skipped"
		statusOpts.Summary = "is skipping this commit."
	case "neutral":
		statusOpts.Title = "Unknown"
		statusOpts.Summary = "doesn't know what happened with this commit."
	}

	if statusOpts.Status == "in_progress" {
		statusOpts.Title = "CI has Started"
		statusOpts.Summary = "is running."
	}

	onPr := ""
	if statusOpts.OriginalPipelineRunName != "" {
		onPr = "/" + statusOpts.OriginalPipelineRunName
	}
	statusOpts.Summary = fmt.Sprintf("%s%s %s", pacopts.ApplicationName, onPr, statusOpts.Summary)

	// If we have an installationID which mean we have a github apps and we can use the checkRun API
	if runevent.InstallationID > 0 {
		return v.getOrUpdateCheckRunStatus(ctx, tekton, runevent, pacopts, statusOpts)
	}

	// Otherwise use the update status commit API
	return v.createStatusCommit(ctx, runevent, pacopts, statusOpts)
}
