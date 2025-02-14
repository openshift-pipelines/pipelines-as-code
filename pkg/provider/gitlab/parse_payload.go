package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {
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

	// Remove the " Hook" suffix so looks better in status, and since we don't
	// really use it anymore we good to do whatever we want with it for
	// cosmetics.
	processedEvent := info.NewEvent()
	processedEvent.EventType = strings.ReplaceAll(event, " Hook", "")
	processedEvent.Event = eventInt
	switch gitEvent := eventInt.(type) {
	case *gitlab.MergeEvent:
		// Organization:  event.GetRepo().GetOwner().GetLogin(),
		processedEvent.Sender = gitEvent.User.Username
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.ObjectAttributes.LastCommit.ID
		processedEvent.SHAURL = gitEvent.ObjectAttributes.LastCommit.URL
		processedEvent.SHATitle = gitEvent.ObjectAttributes.LastCommit.Title
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
		processedEvent.SourceProjectID = gitEvent.ObjectAttributes.SourceProjectID
		processedEvent.TargetProjectID = gitEvent.Project.ID

		processedEvent.TriggerTarget = triggertype.PullRequest
		processedEvent.EventType = strings.ReplaceAll(event, " Hook", "")

		// This is a label update, like adding or removing a label from a MR.
		if gitEvent.Changes.Labels.Current != nil {
			processedEvent.EventType = triggertype.LabelUpdate.String()
		}
		for _, label := range gitEvent.Labels {
			processedEvent.PullRequestLabel = append(processedEvent.PullRequestLabel, label.Title)
		}
		if gitEvent.ObjectAttributes.Action == "close" {
			processedEvent.TriggerTarget = triggertype.PullRequestClosed
		}
	case *gitlab.TagEvent:
		// GitLab sends same event for both Tag creation and deletion i.e. "Tag Push Hook".
		// if gitEvent.After is containing all zeros and gitEvent.CheckoutSHA is empty
		// it is Delete "Tag Push Hook".
		if isZeroSHA(gitEvent.After) && gitEvent.CheckoutSHA == "" {
			return nil, fmt.Errorf("event Delete %s is not supported", event)
		}

		// sometime in gitlab tag push event contains no commit
		// in this case we're not supposed to process the event.
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this %s event", event)
		}

		lastCommitIdx := len(gitEvent.Commits) - 1
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
		processedEvent.TriggerTarget = triggertype.Push
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		v.targetProjectID = gitEvent.ProjectID
		v.sourceProjectID = gitEvent.ProjectID
		v.userID = gitEvent.UserID
		processedEvent.SourceProjectID = gitEvent.ProjectID
		processedEvent.TargetProjectID = gitEvent.ProjectID
		processedEvent.EventType = strings.ReplaceAll(event, " Hook", "")
	case *gitlab.PushEvent:
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this push event")
		}
		lastCommitIdx := len(gitEvent.Commits) - 1
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
		processedEvent.TriggerTarget = triggertype.Push
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		v.targetProjectID = gitEvent.ProjectID
		v.sourceProjectID = gitEvent.ProjectID
		v.userID = gitEvent.UserID
		processedEvent.SourceProjectID = gitEvent.ProjectID
		processedEvent.TargetProjectID = gitEvent.ProjectID
		processedEvent.EventType = strings.ReplaceAll(event, " Hook", "")
	case *gitlab.MergeCommentEvent:
		processedEvent.Sender = gitEvent.User.Username
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.MergeRequest.LastCommit.ID
		processedEvent.SHAURL = gitEvent.MergeRequest.LastCommit.URL
		processedEvent.SHATitle = gitEvent.MergeRequest.LastCommit.Title
		processedEvent.BaseBranch = gitEvent.MergeRequest.TargetBranch
		processedEvent.HeadBranch = gitEvent.MergeRequest.SourceBranch
		processedEvent.BaseURL = gitEvent.MergeRequest.Target.WebURL
		processedEvent.HeadURL = gitEvent.MergeRequest.Source.WebURL

		opscomments.SetEventTypeAndTargetPR(processedEvent, gitEvent.ObjectAttributes.Note)
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		processedEvent.TriggerTarget = triggertype.PullRequest

		processedEvent.PullRequestNumber = gitEvent.MergeRequest.IID
		v.targetProjectID = gitEvent.MergeRequest.TargetProjectID
		v.sourceProjectID = gitEvent.MergeRequest.SourceProjectID
		v.userID = gitEvent.User.ID
		processedEvent.SourceProjectID = gitEvent.MergeRequest.SourceProjectID
		processedEvent.TargetProjectID = gitEvent.MergeRequest.TargetProjectID
	default:
		return nil, fmt.Errorf("event %s is not supported", event)
	}

	v.repoURL = processedEvent.URL
	return processedEvent, nil
}

func isZeroSHA(sha string) bool {
	return sha == "0000000000000000000000000000000000000000"
}
