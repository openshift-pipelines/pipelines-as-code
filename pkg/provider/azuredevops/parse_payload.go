package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/servicehooks"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	types "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops/types"
)

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, req *http.Request, payload string) (*info.Event, error) {
	var genericEvent servicehooks.Event
	err := json.Unmarshal([]byte(payload), &genericEvent)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling payload into Event: %w", err)
	}

	if genericEvent.EventType == nil {
		return nil, fmt.Errorf("event type is nil")
	}

	processedEvent := info.NewEvent()

	processedEvent.EventType = req.Header.Get("X-Azure-DevOps-EventType")

	resourceBytes, err := json.Marshal(genericEvent.Resource)
	if err != nil {
		return nil, fmt.Errorf("error marshalling resource: %w", err)
	}

	switch processedEvent.EventType {
	case "git.push":

		var pushEvent types.PushEventResource
		if err := json.Unmarshal(resourceBytes, &pushEvent); err != nil {
			return nil, fmt.Errorf("error unmarshalling push event resource: %w", err)
		}
		if len(pushEvent.Commits) > 0 {
			processedEvent.SHA = pushEvent.Commits[0].CommitID
			processedEvent.SHAURL = pushEvent.Commits[0].URL
			processedEvent.SHATitle = pushEvent.Commits[0].Comment
		}

		processedEvent.Sender = pushEvent.PushedBy.ID
		baseURL, err := ExtractBaseURL(pushEvent.Repository.RemoteURL)
		if err != nil {
			return nil, fmt.Errorf("not able to extract organization url")
		}
		processedEvent.Organization = baseURL

		processedEvent.Repository = pushEvent.Repository.RemoteURL
		processedEvent.RepositoryID = pushEvent.Repository.ID
		processedEvent.ProjectID = pushEvent.Repository.Project.ID
		processedEvent.URL = pushEvent.Repository.RemoteURL
		processedEvent.DefaultBranch = pushEvent.Repository.DefaultBranch
		processedEvent.TriggerTarget = triggertype.Push
		processedEvent.BaseURL = pushEvent.Repository.URL
		processedEvent.HeadURL = pushEvent.Repository.URL
		if len(pushEvent.RefUpdates) > 0 {
			branchName := ExtractBranchName(pushEvent.RefUpdates[0].Name)
			processedEvent.BaseBranch = branchName
			processedEvent.HeadBranch = branchName
		}

	case "git.pullrequest.created", "git.pullrequest.updated":
		var prEvent types.PullRequestEventResource
		if err := json.Unmarshal(resourceBytes, &prEvent); err != nil {
			return nil, fmt.Errorf("error unmarshalling pull request event resource: %w", err)
		}

		processedEvent.PullRequestNumber = prEvent.PullRequestID
		processedEvent.PullRequestTitle = prEvent.Title
		processedEvent.SHA = prEvent.LastMergeSourceCommit.CommitID
		processedEvent.SHAURL = prEvent.LastMergeSourceCommit.URL
		processedEvent.SHATitle = prEvent.LastMergeSourceCommit.Comment

		processedEvent.BaseBranch = ExtractBranchName(prEvent.TargetRefName)
		processedEvent.HeadBranch = ExtractBranchName(prEvent.SourceRefName)
		processedEvent.DefaultBranch = prEvent.Repository.DefaultBranch

		processedEvent.TriggerTarget = triggertype.PullRequest
		remoteURL := *prEvent.Repository.WebURL
		baseURL, err := ExtractBaseURL(remoteURL)
		if err != nil {
			return nil, fmt.Errorf("not able to extract organization url")
		}
		processedEvent.Organization = baseURL
		processedEvent.Repository = *prEvent.Repository.WebURL
		processedEvent.RepositoryID = prEvent.Repository.ID
		processedEvent.ProjectID = prEvent.Repository.Project.ID
		processedEvent.URL = *prEvent.Repository.WebURL
		processedEvent.Sender = prEvent.CreatedBy.ID

	case "git.pullrequest.comment":

		var prEvent types.PullRequestCommentEventResource
		if err := json.Unmarshal(resourceBytes, &prEvent); err != nil {
			return nil, fmt.Errorf("error unmarshalling pull request event resource: %w", err)
		}

		processedEvent.PullRequestNumber = prEvent.PullRequest.PullRequestID
		processedEvent.PullRequestTitle = prEvent.PullRequest.Title
		processedEvent.SHA = prEvent.PullRequest.LastMergeSourceCommit.CommitID
		processedEvent.SHAURL = prEvent.PullRequest.LastMergeSourceCommit.URL
		processedEvent.SHATitle = prEvent.PullRequest.LastMergeSourceCommit.Comment

		processedEvent.BaseBranch = ExtractBranchName(prEvent.PullRequest.TargetRefName)
		processedEvent.HeadBranch = ExtractBranchName(prEvent.PullRequest.SourceRefName)
		processedEvent.DefaultBranch = prEvent.PullRequest.Repository.DefaultBranch

		remoteURL := *prEvent.PullRequest.Repository.WebURL
		processedEvent.TriggerTarget = triggertype.PullRequest
		baseURL, err := ExtractBaseURL(remoteURL)
		if err != nil {
			return nil, fmt.Errorf("not able to extract organization url")
		}
		processedEvent.Organization = baseURL
		processedEvent.Repository = *prEvent.PullRequest.Repository.WebURL
		processedEvent.RepositoryID = prEvent.PullRequest.Repository.ID
		processedEvent.ProjectID = prEvent.PullRequest.Repository.Project.ID
		processedEvent.URL = *prEvent.PullRequest.Repository.WebURL
		processedEvent.Sender = prEvent.PullRequest.CreatedBy.ID
		opscomments.SetEventTypeAndTargetPR(processedEvent, prEvent.Comment.Content)

	default:
		return nil, fmt.Errorf("event type %s is not supported", *genericEvent.EventType)
	}

	return processedEvent, nil
}

// ExtractBranchName extracts the branch name from a full ref name.
// E.g., "refs/heads/master" -> "master".
func ExtractBranchName(refName string) string {
	parts := strings.Split(refName, "/")
	if len(parts) > 2 {
		return parts[len(parts)-1] // Get the last part which should be the branch name
	}
	return refName // Return as-is if the format is unexpected
}

func ExtractBaseURL(url string) (string, error) {
	re := regexp.MustCompile(`^(https://dev\.azure\.com/[^/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("base URL could not be extracted")
	}
	return matches[1], nil
}
