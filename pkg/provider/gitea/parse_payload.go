package gitea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	giteaStructs "code.gitea.io/gitea/modules/structs"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
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
	case *giteaStructs.PullRequestPayload:
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
		processedEvent.TriggerTarget = triggertype.PullRequest
		processedEvent.EventType = triggertype.PullRequest.String()
		if provider.Valid(string(gitEvent.Action), []string{pullRequestLabelUpdated}) {
			processedEvent.EventType = string(triggertype.LabelUpdate)
		}
		for _, label := range gitEvent.PullRequest.Labels {
			processedEvent.PullRequestLabel = append(processedEvent.PullRequestLabel, label.Name)
		}
	case *giteaStructs.PushPayload:
		processedEvent = info.NewEvent()
		processedEvent.SHA = gitEvent.HeadCommit.ID
		if processedEvent.SHA == "" {
			processedEvent.SHA = gitEvent.Before
		}
		processedEvent.SHAURL = gitEvent.HeadCommit.URL
		processedEvent.SHATitle = gitEvent.HeadCommit.Message
		processedEvent.Organization = gitEvent.Repo.Owner.UserName
		processedEvent.Repository = gitEvent.Repo.Name
		processedEvent.DefaultBranch = gitEvent.Repo.DefaultBranch
		processedEvent.URL = gitEvent.Repo.HTMLURL
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.EventType = eventType
		processedEvent.HeadBranch = processedEvent.BaseBranch // in push events Head Branch is the same as Basebranch
		processedEvent.BaseURL = gitEvent.Repo.HTMLURL
		processedEvent.HeadURL = processedEvent.BaseURL // in push events Head URL is the same as BaseURL
		processedEvent.TriggerTarget = "push"
	case *giteaStructs.IssueCommentPayload:
		if gitEvent.Issue.PullRequest == nil {
			return info.NewEvent(), fmt.Errorf("issue comment is not coming from a pull_request")
		}
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.Repository.Owner.UserName
		processedEvent.Repository = gitEvent.Repository.Name
		processedEvent.Sender = gitEvent.Sender.UserName
		processedEvent.TriggerTarget = triggertype.PullRequest
		opscomments.SetEventTypeAndTargetPR(processedEvent, gitEvent.Comment.Body)
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
