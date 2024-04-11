package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/servicehooks"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	types "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops/types"
)

func (v *Provider) ParsePayload(_ context.Context, _ *params.Run, request *http.Request,
	payload string,
) (*info.Event, error) {

	var genericEvent servicehooks.Event
	err := json.Unmarshal([]byte(payload), &genericEvent)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling payload into Event: %v", err)
	}

	if genericEvent.EventType == nil {
		return nil, fmt.Errorf("event type is nil")
	}

	processedEvent := info.NewEvent()
	processedEvent.EventType = *genericEvent.EventType

	resourceBytes, err := json.Marshal(genericEvent.Resource)
	if err != nil {
		return nil, fmt.Errorf("error marshalling resource: %v", err)
	}

	switch *genericEvent.EventType {
	case "git.push":

		var pushEvent types.PushEventResource
		if err := json.Unmarshal(resourceBytes, &pushEvent); err != nil {
			return nil, fmt.Errorf("error unmarshalling push event resource: %v", err)
		}
		if len(pushEvent.Commits) > 0 {
			processedEvent.SHA = pushEvent.Commits[0].CommitId
			processedEvent.SHAURL = pushEvent.Commits[0].Url
			processedEvent.SHATitle = pushEvent.Commits[0].Comment
		}

		processedEvent.EventType = *genericEvent.EventType
		processedEvent.Sender = pushEvent.PushedBy.DisplayName
		baseURL, err := extractBaseURL(pushEvent.Repository.RemoteUrl)
		if err != nil {
			fmt.Println("Error:", err)
			return nil, fmt.Errorf("not able to extract organization url")
		}
		processedEvent.Organization = baseURL
		fmt.Println("Base URL:", baseURL)

		processedEvent.Repository = pushEvent.Repository.RemoteUrl
		processedEvent.RepositoryId = pushEvent.Repository.Id
		processedEvent.ProjectId = pushEvent.Repository.Project.Id
		processedEvent.URL = pushEvent.Repository.RemoteUrl
		processedEvent.DefaultBranch = pushEvent.Repository.DefaultBranch
		processedEvent.TriggerTarget = triggertype.Push
		// Assuming the repository URL can serve as both BaseURL and HeadURL for viewing purposes
		processedEvent.BaseURL = pushEvent.Repository.Url // or it could be remoteUrl or it could be other; need to verify
		processedEvent.HeadURL = pushEvent.Repository.Url // or it could be remoteUrl or it could be othe; need to verify
		if len(pushEvent.RefUpdates) > 0 {
			branchName := ExtractBranchName(pushEvent.RefUpdates[0].Name)
			processedEvent.BaseBranch = branchName
			processedEvent.HeadBranch = branchName
		}

	case "git.pullrequest.created":
		var prEvent types.PullRequestEventResource
		if err := json.Unmarshal(resourceBytes, &prEvent); err != nil {
			return nil, fmt.Errorf("error unmarshalling pull request event resource: %v", err)
		}

		processedEvent.EventType = *genericEvent.EventType
		processedEvent.PullRequestNumber = prEvent.PullRequestId
		processedEvent.PullRequestTitle = prEvent.Title
		processedEvent.SHA = prEvent.LastMergeSourceCommit.CommitId
		processedEvent.SHAURL = prEvent.LastMergeSourceCommit.Url
		processedEvent.SHATitle = prEvent.LastMergeSourceCommit.Comment

		// Extract branch names from the ref names
		// Azure DevOps ref names are full references (refs/heads/branchName), so we'll extract the branch name
		processedEvent.BaseBranch = ExtractBranchName(prEvent.TargetRefName)
		processedEvent.HeadBranch = ExtractBranchName(prEvent.SourceRefName)
		processedEvent.DefaultBranch = prEvent.Repository.DefaultBranch

		// Constructing URLs
		remoteUrl := *prEvent.Repository.WebUrl
		processedEvent.BaseURL = fmt.Sprintf("%s?version=GB%s", remoteUrl, processedEvent.BaseBranch)
		processedEvent.HeadURL = fmt.Sprintf("%s?version=GB%s", remoteUrl, processedEvent.HeadBranch)

		processedEvent.TriggerTarget = triggertype.PullRequest

		baseURL, err := extractBaseURL(remoteUrl)
		if err != nil {
			fmt.Println("Error:", err)
			return nil, fmt.Errorf("not able to extract organization url")
		}
		processedEvent.Organization = baseURL
		processedEvent.Repository = *prEvent.Repository.WebUrl
		processedEvent.RepositoryId = prEvent.Repository.Id
		processedEvent.ProjectId = prEvent.Repository.Project.Id
		processedEvent.URL = *prEvent.Repository.WebUrl
		processedEvent.Sender = prEvent.CreatedBy.DisplayName
	default:
		return nil, fmt.Errorf("event type %s is not supported", *genericEvent.EventType)
	}

	return processedEvent, nil
}

// ExtractBranchName extracts the branch name from a full ref name.
// E.g., "refs/heads/master" -> "master"
func ExtractBranchName(refName string) string {
	parts := strings.Split(refName, "/")
	if len(parts) > 2 {
		return parts[len(parts)-1] // Get the last part which should be the branch name
	}
	return refName // Return as-is if the format is unexpected
}

func extractBaseURL(url string) (string, error) {
	re := regexp.MustCompile(`^(https://dev\.azure\.com/[^/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("base URL could not be extracted")
	}
	return matches[1], nil
}
