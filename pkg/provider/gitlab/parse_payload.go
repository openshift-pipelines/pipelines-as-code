package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/xanzy/go-gitlab"
)

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {
	// TODO: parse request to figure out which event
	var processedEvent *info.Event

	event := request.Header.Get("X-Gitlab-Event")
	if event == "" {
		return nil, fmt.Errorf("failed to find event type in request header")
	}

	payloadB := []byte(payload)
	eventInt, err := gitlab.ParseWebhook(gitlab.EventType(event), payloadB)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payloadB, &eventInt)

	switch gitEvent := eventInt.(type) {
	case *gitlab.MergeEvent:
		processedEvent = info.NewEvent()
		// Organization:  event.GetRepo().GetOwner().GetLogin(),
		processedEvent.Sender = gitEvent.User.Username
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.ObjectAttributes.LastCommit.ID
		processedEvent.SHAURL = gitEvent.ObjectAttributes.LastCommit.URL
		processedEvent.SHATitle = gitEvent.ObjectAttributes.Title
		processedEvent.HeadBranch = gitEvent.ObjectAttributes.SourceBranch
		processedEvent.BaseBranch = gitEvent.ObjectAttributes.TargetBranch
		processedEvent.HeadURL = gitEvent.ObjectAttributes.Source.WebURL
		processedEvent.BaseURL = gitEvent.ObjectAttributes.Target.WebURL
		processedEvent.PullRequestNumber = gitEvent.ObjectAttributes.IID
		processedEvent.PullRequestTitle = gitEvent.ObjectAttributes.Title
		v.targetProjectID = gitEvent.Project.ID
		v.sourceProjectID = gitEvent.ObjectAttributes.SourceProjectID
		v.userID = gitEvent.User.ID

		v.pathWithNamespace = gitEvent.ObjectAttributes.Target.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		processedEvent.TriggerTarget = "pull_request"
		processedEvent.SourceProjectID = gitEvent.ObjectAttributes.SourceProjectID
		processedEvent.TargetProjectID = gitEvent.Project.ID
	case *gitlab.TagEvent:
		lastCommitIdx := len(gitEvent.Commits) - 1
		processedEvent = info.NewEvent()
		processedEvent.Sender = gitEvent.UserUsername
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.Commits[lastCommitIdx].ID
		processedEvent.SHAURL = gitEvent.Commits[lastCommitIdx].URL
		processedEvent.SHATitle = gitEvent.Commits[lastCommitIdx].Title
		processedEvent.HeadBranch = gitEvent.Ref
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.HeadURL = gitEvent.Project.WebURL
		processedEvent.BaseURL = processedEvent.HeadURL
		processedEvent.TriggerTarget = "push"
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		v.targetProjectID = gitEvent.ProjectID
		v.sourceProjectID = gitEvent.ProjectID
		v.userID = gitEvent.UserID
		processedEvent.SourceProjectID = gitEvent.ProjectID
		processedEvent.TargetProjectID = gitEvent.ProjectID
	case *gitlab.PushEvent:
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this push event")
		}
		lastCommitIdx := len(gitEvent.Commits) - 1
		processedEvent = info.NewEvent()
		processedEvent.Sender = gitEvent.UserUsername
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.Commits[lastCommitIdx].ID
		processedEvent.SHAURL = gitEvent.Commits[lastCommitIdx].URL
		processedEvent.SHATitle = gitEvent.Commits[lastCommitIdx].Title
		processedEvent.HeadBranch = gitEvent.Ref
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.HeadURL = gitEvent.Project.WebURL
		processedEvent.BaseURL = processedEvent.HeadURL
		processedEvent.TriggerTarget = "push"
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		v.targetProjectID = gitEvent.ProjectID
		v.sourceProjectID = gitEvent.ProjectID
		v.userID = gitEvent.UserID
		processedEvent.SourceProjectID = gitEvent.ProjectID
		processedEvent.TargetProjectID = gitEvent.ProjectID
	case *gitlab.MergeCommentEvent:
		processedEvent = info.NewEvent()
		processedEvent.Sender = gitEvent.User.Username
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.MergeRequest.LastCommit.ID
		processedEvent.SHAURL = gitEvent.MergeRequest.LastCommit.URL
		// TODO: change this back to Title when we get this pr available merged https://github.com/xanzy/go-gitlab/pull/1406/files
		processedEvent.SHATitle = gitEvent.MergeRequest.LastCommit.Message
		processedEvent.BaseBranch = gitEvent.MergeRequest.TargetBranch
		processedEvent.HeadBranch = gitEvent.MergeRequest.SourceBranch
		processedEvent.BaseURL = gitEvent.MergeRequest.Target.WebURL
		processedEvent.HeadURL = gitEvent.MergeRequest.Source.WebURL
		// if it is a /test or /retest comment with pipelinerun name figure out the pipelineRun name
		if provider.IsTestRetestComment(gitEvent.ObjectAttributes.Note) {
			processedEvent.TargetTestPipelineRun = provider.GetPipelineRunFromTestComment(gitEvent.ObjectAttributes.Note)
		}
		if provider.IsCancelComment(gitEvent.ObjectAttributes.Note) {
			processedEvent.TargetCancelPipelineRun = provider.GetPipelineRunFromCancelComment(gitEvent.ObjectAttributes.Note)
		}

		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		processedEvent.TriggerTarget = "pull_request"

		processedEvent.PullRequestNumber = gitEvent.MergeRequest.IID
		v.targetProjectID = gitEvent.MergeRequest.TargetProjectID
		v.sourceProjectID = gitEvent.MergeRequest.SourceProjectID
		v.userID = gitEvent.User.ID
		processedEvent.SourceProjectID = gitEvent.MergeRequest.SourceProjectID
		processedEvent.TargetProjectID = gitEvent.MergeRequest.TargetProjectID
	default:
		return nil, fmt.Errorf("event %s is not supported", event)
	}

	processedEvent.Event = eventInt

	// Remove the " Hook" suffix so looks better in status, and since we don't
	// really use it anymore we good to do whatever we want with it for
	// cosmetics.
	processedEvent.EventType = strings.ReplaceAll(event, " Hook", "")

	v.repoURL = processedEvent.URL
	return processedEvent, nil
}
