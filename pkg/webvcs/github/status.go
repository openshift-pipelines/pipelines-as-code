package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
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

// createCheckRunStatus create a status via the checkRun API, which is only
// available with Github apps tokens.
func (v *VCS) createCheckRunStatus(ctx context.Context, runevent *info.Event, pacopts info.PacOpts, status webvcs.StatusOpts) error {
	now := github.Timestamp{Time: time.Now()}
	if runevent.CheckRunID == nil {
		now := github.Timestamp{Time: time.Now()}
		checkrunoption := github.CreateCheckRunOptions{
			Name:       pacopts.ApplicationName,
			HeadSHA:    runevent.SHA,
			Status:     github.String("in_progress"),
			DetailsURL: github.String(pacopts.LogURL),
			StartedAt:  &now,
		}

		checkRun, _, err := v.Client.Checks.CreateCheckRun(ctx, runevent.Owner, runevent.Repository, checkrunoption)
		if err != nil {
			return err
		}
		runevent.CheckRunID = checkRun.ID
	}

	checkRunOutput := &github.CheckRunOutput{
		Title:   &status.Title,
		Summary: &status.Summary,
		Text:    &status.Text,
	}

	opts := github.UpdateCheckRunOptions{
		Name:   pacopts.ApplicationName,
		Status: &status.Status,
		Output: checkRunOutput,
	}

	if status.DetailsURL != "" {
		opts.DetailsURL = &status.DetailsURL
	}

	// Only set completed-at if conclusion is set (which means finished)
	if status.Conclusion != "" && status.Conclusion != "pending" {
		opts.CompletedAt = &now
		opts.Conclusion = &status.Conclusion
	}

	_, _, err := v.Client.Checks.UpdateCheckRun(ctx, runevent.Owner, runevent.Repository, *runevent.CheckRunID, opts)
	return err
}

// createStatusCommit use the classic/old statuses API which is available when we
// don't have a github app token
func (v *VCS) createStatusCommit(ctx context.Context, runevent *info.Event, pacopts info.PacOpts, status webvcs.StatusOpts) error {
	now := time.Now()
	switch status.Conclusion {
	case "skipped":
		status.Conclusion = "success" // We don't have a choice than setting as succes, no pending here.
	case "neutral":
		status.Conclusion = "success" // We don't have a choice than setting as succes, no pending here.
	}
	if status.Status == "in_progress" {
		status.Conclusion = "pending"
	}

	ghstatus := &github.RepoStatus{
		State:       github.String(status.Conclusion),
		TargetURL:   github.String(status.DetailsURL),
		Description: github.String(status.Title),
		Context:     github.String(pacopts.ApplicationName),
		CreatedAt:   &now,
	}

	_, _, err := v.Client.Repositories.CreateStatus(ctx, runevent.Owner, runevent.Repository, runevent.SHA, ghstatus)
	if err != nil {
		return err
	}
	if status.Status == "completed" && status.Text != "" && runevent.EventType == "pull_request" {
		payloadevent, ok := runevent.Event.(*github.PullRequestEvent)
		if !ok {
			return fmt.Errorf("bad event??? %w", err)
		}
		_, _, err = v.Client.Issues.CreateComment(ctx, runevent.Owner, runevent.Repository,
			payloadevent.GetPullRequest().GetNumber(),
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

func (v *VCS) CreateStatus(ctx context.Context, runevent *info.Event, pacopts info.PacOpts, status webvcs.StatusOpts) error {
	if pacopts.VCSAPIURL == "" || pacopts.VCSToken == "" {
		return fmt.Errorf("cannot set status on github no token or url set")
	}

	switch status.Conclusion {
	case "success":
		status.Title = "✅ Success"
		status.Summary = fmt.Sprintf("%s has <b>successfully</b> validated your commit.", pacopts.ApplicationName)
	case "failure":
		status.Title = "❌ Failed"
		status.Summary = fmt.Sprintf("%s has <b>failed</b>.", pacopts.ApplicationName)
	case "skipped":
		status.Title = "➖ Skipped"
		status.Summary = fmt.Sprintf("%s is skipping this commit.", pacopts.ApplicationName)
	case "neutral":
		status.Title = "❓ Unknown"
		status.Summary = fmt.Sprintf("%s doesn't know what happened with this commit.", pacopts.ApplicationName)
	}

	if status.Status == "in_progress" {
		status.Title = "CI has Started"
		status.Summary = fmt.Sprintf("%s is running.", pacopts.ApplicationName)
	}

	if pacopts.VCSInfoFromRepo {
		return v.createStatusCommit(ctx, runevent, pacopts, status)
	}
	return v.createCheckRunStatus(ctx, runevent, pacopts, status)
}
