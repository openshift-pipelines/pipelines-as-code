package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v59/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/action"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	kstatus "github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction/status"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const taskStatusTemplate = `
<table>
  <tr><th>Status</th><th>Duration</th><th>Name</th></tr>

{{- range $taskrun := .TaskRunList }}
<tr>
<td>{{ formatCondition $taskrun.PipelineRunTaskRunStatus.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.PipelineRunTaskRunStatus.Status.StartTime $taskrun.PipelineRunTaskRunStatus.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`

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
	opt := github.ListOptions{PerPage: v.paginedNumber}
	for {
		res, resp, err := v.Client.Checks.ListCheckRunsForRef(ctx, runevent.Organization, runevent.Repository,
			runevent.SHA, &github.ListCheckRunsOptions{
				AppID:       v.ApplicationID,
				ListOptions: opt,
			})
		if err != nil {
			return nil, err
		}

		for _, checkrun := range res.CheckRuns {
			// if it is a Pending approval CheckRun then overwrite it
			if isPendingApprovalCheckrun(checkrun) {
				if v.canIUseCheckrunID(checkrun.ID) {
					return checkrun.ID, nil
				}
			}
			if *checkrun.ExternalID == status.PipelineRunName {
				return checkrun.ID, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return nil, nil
}

func isPendingApprovalCheckrun(run *github.CheckRun) bool {
	if run == nil || run.Output == nil {
		return false
	}
	if run.Output.Title != nil && strings.Contains(*run.Output.Title, "Pending") &&
		run.Output.Summary != nil &&
		strings.Contains(*run.Output.Summary, "is waiting for approval") {
		return true
	}
	return false
}

func (v *Provider) canIUseCheckrunID(checkrunid *int64) bool {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if v.checkRunID == 0 {
		v.checkRunID = *checkrunid
		return true
	}
	return false
}

func (v *Provider) createCheckRunStatus(ctx context.Context, runevent *info.Event, status provider.StatusOpts) (*int64, error) {
	now := github.Timestamp{Time: time.Now()}
	checkrunoption := github.CreateCheckRunOptions{
		Name:       getCheckName(status, &v.pacInfo),
		HeadSHA:    runevent.SHA,
		Status:     github.String("in_progress"),
		DetailsURL: github.String(status.DetailsURL),
		ExternalID: github.String(status.PipelineRunName),
		StartedAt:  &now,
	}

	checkRun, _, err := v.Client.Checks.CreateCheckRun(ctx, runevent.Organization, runevent.Repository, checkrunoption)
	if err != nil {
		return nil, err
	}
	return checkRun.ID, nil
}

func (v *Provider) getFailuresMessageAsAnnotations(ctx context.Context, pr *tektonv1.PipelineRun, pacopts *info.PacOpts) []*github.CheckRunAnnotation {
	annotations := []*github.CheckRunAnnotation{}
	r, err := regexp.Compile(pacopts.ErrorDetectionSimpleRegexp)
	if err != nil {
		v.Logger.Errorf("invalid regexp for filtering failure messages: %v", pacopts.ErrorDetectionSimpleRegexp)
		return annotations
	}
	intf, err := kubeinteraction.NewKubernetesInteraction(v.Run)
	if err != nil {
		v.Logger.Errorf("failed to create kubeinteraction: %v", err)
		return annotations
	}
	taskinfos := kstatus.CollectFailedTasksLogSnippet(ctx, v.Run, intf, pr, int64(pacopts.ErrorDetectionNumberOfLines))
	for _, taskinfo := range taskinfos {
		for _, errline := range strings.Split(taskinfo.LogSnippet, "\n") {
			results := map[string]string{}
			if !r.MatchString(errline) {
				continue
			}
			matches := r.FindStringSubmatch(errline)
			for i, name := range r.SubexpNames() {
				if i != 0 && name != "" {
					results[name] = matches[i]
				}
			}

			// check if we  have file in results
			var linenumber, errmsg, filename string
			var ok bool

			if filename, ok = results["filename"]; !ok {
				v.Logger.Errorf("regexp for filtering failure messages does not contain a filename regexp group: %v", pacopts.ErrorDetectionSimpleRegexp)
				continue
			}
			// remove ./ cause it would bug github otherwise
			filename = strings.TrimPrefix(filename, "./")

			if linenumber, ok = results["line"]; !ok {
				v.Logger.Errorf("regexp for filtering failure messages does not contain a line regexp group: %v", pacopts.ErrorDetectionSimpleRegexp)
				continue
			}

			if errmsg, ok = results["error"]; !ok {
				v.Logger.Errorf("regexp for filtering failure messages does not contain a error regexp group: %v", pacopts.ErrorDetectionSimpleRegexp)
				continue
			}

			ilinenumber, err := strconv.Atoi(linenumber)
			if err != nil {
				// can't do much regexp has probably failed to detect
				v.Logger.Errorf("cannot convert %s as integer: %v", linenumber, err)
				continue
			}
			annotations = append(annotations, &github.CheckRunAnnotation{
				Path:            github.String(filename),
				StartLine:       github.Int(ilinenumber),
				EndLine:         github.Int(ilinenumber),
				AnnotationLevel: github.String("failure"),
				Message:         github.String(errmsg),
			})
		}
	}
	return annotations
}

// getOrUpdateCheckRunStatus create a status via the checkRun API, which is only
// available with Github apps tokens.
func (v *Provider) getOrUpdateCheckRunStatus(ctx context.Context, runevent *info.Event, statusOpts provider.StatusOpts) error {
	var err error
	var checkRunID *int64
	var found bool
	pacopts := &v.pacInfo

	// check if pipelineRun has the label with checkRun-id
	if statusOpts.PipelineRun != nil {
		var id string
		id, found = statusOpts.PipelineRun.GetAnnotations()[keys.CheckRunID]
		if found {
			checkID, err := strconv.Atoi(id)
			if err != nil {
				return fmt.Errorf("api error: cannot convert checkrunid")
			}
			checkRunID = github.Int64(int64(checkID))
		}
	}
	if !found {
		if checkRunID, _ = v.getExistingCheckRunID(ctx, runevent, statusOpts); checkRunID == nil {
			checkRunID, err = v.createCheckRunStatus(ctx, runevent, statusOpts)
			if err != nil {
				return err
			}
		}
		if statusOpts.PipelineRun != nil {
			if _, err := action.PatchPipelineRun(ctx, v.Logger, "checkRunID and logURL", v.Run.Clients.Tekton, statusOpts.PipelineRun, metadataPatch(checkRunID, statusOpts.DetailsURL)); err != nil {
				return err
			}
		}
	}

	text := statusOpts.Text
	checkRunOutput := &github.CheckRunOutput{
		Title:   &statusOpts.Title,
		Summary: &statusOpts.Summary,
	}

	if statusOpts.PipelineRun != nil {
		if pacopts.ErrorDetection {
			checkRunOutput.Annotations = v.getFailuresMessageAsAnnotations(ctx, statusOpts.PipelineRun, pacopts)
		}
	}

	checkRunOutput.Text = github.String(text)

	opts := github.UpdateCheckRunOptions{
		Name:   getCheckName(statusOpts, pacopts),
		Status: github.String(statusOpts.Status),
		Output: checkRunOutput,
	}
	if statusOpts.PipelineRunName != "" {
		opts.ExternalID = github.String(statusOpts.PipelineRunName)
	}
	if statusOpts.DetailsURL != "" {
		opts.DetailsURL = &statusOpts.DetailsURL
	}

	// Only set completed-at if conclusion is set (which means finished)
	if statusOpts.Conclusion != "" && statusOpts.Conclusion != "pending" {
		opts.CompletedAt = &github.Timestamp{Time: time.Now()}
		opts.Conclusion = &statusOpts.Conclusion
	}
	if isPipelineRunCancelledOrStopped(statusOpts.PipelineRun) {
		opts.Conclusion = github.String("cancelled")
	}

	_, _, err = v.Client.Checks.UpdateCheckRun(ctx, runevent.Organization, runevent.Repository, *checkRunID, opts)
	return err
}

func isPipelineRunCancelledOrStopped(run *tektonv1.PipelineRun) bool {
	if run == nil {
		return false
	}
	if run.IsCancelled() || run.IsGracefullyCancelled() || run.IsGracefullyStopped() {
		return true
	}
	return false
}

func metadataPatch(checkRunID *int64, logURL string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				keys.CheckRunID: strconv.FormatInt(*checkRunID, 10),
			},
			"annotations": map[string]string{
				keys.LogURL:     logURL,
				keys.CheckRunID: strconv.FormatInt(*checkRunID, 10),
			},
		},
	}
}

// createStatusCommit use the classic/old statuses API which is available when we
// don't have a github app token.
func (v *Provider) createStatusCommit(ctx context.Context, runevent *info.Event, status provider.StatusOpts) error {
	var err error
	now := time.Now()
	switch status.Conclusion {
	case "neutral":
		status.Conclusion = "success" // We don't have a choice than setting as success, no pending here.
	case "pending":
		if status.Title != "" {
			status.Conclusion = "pending"
		}
	}
	if status.Status == "in_progress" {
		status.Conclusion = "pending"
	}

	ghstatus := &github.RepoStatus{
		State:       github.String(status.Conclusion),
		TargetURL:   github.String(status.DetailsURL),
		Description: github.String(status.Title),
		Context:     github.String(getCheckName(status, &v.pacInfo)),
		CreatedAt:   &github.Timestamp{Time: now},
	}

	if _, _, err := v.Client.Repositories.CreateStatus(ctx,
		runevent.Organization, runevent.Repository, runevent.SHA, ghstatus); err != nil {
		return err
	}
	if (status.Status == "completed" || (status.Status == "queued" && status.Title == "Pending approval")) && status.Text != "" && runevent.EventType == triggertype.PullRequest.String() {
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

func (v *Provider) CreateStatus(ctx context.Context, runevent *info.Event, statusOpts provider.StatusOpts) error {
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
	case "pending":
		// for concurrency set title as pending
		if statusOpts.Title == "" {
			statusOpts.Title = "Pending"
			statusOpts.Summary = "is skipping this commit."
		} else {
			// for unauthorized user set title as Pending approval
			statusOpts.Summary = "is waiting for approval."
		}
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
	statusOpts.Summary = fmt.Sprintf("%s%s %s", v.pacInfo.ApplicationName, onPr, statusOpts.Summary)

	// If we have an installationID which mean we have a github apps and we can use the checkRun API
	if runevent.InstallationID > 0 {
		return v.getOrUpdateCheckRunStatus(ctx, runevent, statusOpts)
	}

	// Otherwise use the update status commit API
	return v.createStatusCommit(ctx, runevent, statusOpts)
}
