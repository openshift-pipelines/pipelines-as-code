package gitea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	giteastruct "code.gitea.io/gitea/modules/structs"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {
	// TODO: parse request to figure out which event
	var processedEvent *info.Event

	eventType := request.Header.Get("X-Gitea-Event-Type")
	if eventType == "" {
		return nil, fmt.Errorf("failed to find event type in request header")
	}

	payloadB := []byte(payload)
	eventInt, err := parseWebhook(whEventType(eventType), payloadB)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payloadB, &eventInt)

	switch gitEvent := eventInt.(type) {
	case *giteastruct.PullRequestPayload:
		processedEvent = info.NewEvent()
		// // Organization:  event.GetRepo().GetOwner().GetLogin(),
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.DefaultBranch = gitEvent.Repository.DefaultBranch
		processedEvent.URL = gitEvent.Repository.HTMLURL
		processedEvent.SHA = gitEvent.PullRequest.Head.Sha
		processedEvent.SHAURL = fmt.Sprintf("%s/commit/%s", gitEvent.PullRequest.HTMLURL, processedEvent.SHA)
		processedEvent.HeadBranch = gitEvent.PullRequest.Head.Ref
		processedEvent.BaseBranch = gitEvent.PullRequest.Base.Ref
		processedEvent.HeadURL = gitEvent.PullRequest.Head.Repository.HTMLURL
		processedEvent.BaseURL = gitEvent.PullRequest.Base.Repository.HTMLURL
		processedEvent.PullRequestNumber = int(gitEvent.Index)
		processedEvent.PullRequestTitle = gitEvent.PullRequest.Title
		processedEvent.Organization = gitEvent.Repository.Owner.UserName
		processedEvent.Repository = gitEvent.Repository.Name
		processedEvent.TriggerTarget = "pull_request"
		processedEvent.EventType = "pull_request"
	case *giteastruct.PushPayload:
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this push event")
		}
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.Repo.Owner.UserName
		processedEvent.Repository = gitEvent.Repo.Name
		processedEvent.DefaultBranch = gitEvent.Repo.DefaultBranch
		processedEvent.URL = gitEvent.Repo.HTMLURL
		processedEvent.SHA = gitEvent.HeadCommit.ID
		if processedEvent.SHA == "" {
			processedEvent.SHA = gitEvent.Before
		}
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.SHAURL = gitEvent.HeadCommit.URL
		processedEvent.SHATitle = gitEvent.HeadCommit.Message
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.EventType = eventType
		processedEvent.HeadBranch = processedEvent.BaseBranch // in push events Head Branch is the same as Basebranch
		processedEvent.BaseURL = gitEvent.Repo.HTMLURL
		processedEvent.HeadURL = processedEvent.BaseURL // in push events Head URL is the same as BaseURL
		processedEvent.TriggerTarget = "push"
	case *giteastruct.IssueCommentPayload:
		if gitEvent.Issue.PullRequest == nil {
			return info.NewEvent(), fmt.Errorf("issue comment is not coming from a pull_request")
		}
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.Repository.Owner.UserName
		processedEvent.Repository = gitEvent.Repository.Name
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.TriggerTarget = "pull_request"
		processedEvent.EventType = "pull_request"

		if provider.IsTestRetestComment(gitEvent.Comment.Body) {
			processedEvent.TargetTestPipelineRun = provider.GetPipelineRunFromTestComment(gitEvent.Comment.Body)
		}
		if provider.IsCancelComment(gitEvent.Comment.Body) {
			processedEvent.CancelPipelineRuns = true
			processedEvent.TargetCancelPipelineRun = provider.GetPipelineRunFromCancelComment(gitEvent.Comment.Body)
		}
		processedEvent.PullRequestNumber, err = convertPullRequestURLtoNumber(gitEvent.Issue.URL)
		if err != nil {
			return nil, err
		}
		processedEvent.URL = gitEvent.Repository.HTMLURL
		processedEvent.DefaultBranch = gitEvent.Repository.DefaultBranch
	default:
		return nil, fmt.Errorf("event %s is not supported", eventType)
	}

	processedEvent.Event = eventInt
	return processedEvent, nil
}
