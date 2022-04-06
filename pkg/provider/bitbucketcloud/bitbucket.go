package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"go.uber.org/zap"
)

type Provider struct {
	Client        *bitbucket.Client
	Token, APIURL *string
	Username      *string
}

const taskStatusTemplate = `| **Status** | **Duration** | **Name** |
| --- | --- | --- |
{{range $taskrun := .TaskRunList }}|{{ formatCondition $taskrun.Status.Conditions }}|{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}|{{ $taskrun.ConsoleLogURL }}|
{{ end }}`

func (v *Provider) Validate(ctx context.Context, params *params.Run, event *info.Event) error {
	return nil
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         bitbucket.DEFAULT_BITBUCKET_API_BASE_URL,
	}
}

func (v *Provider) CreateStatus(_ context.Context, event *info.Event, pacopts *info.PacOpts, statusopts provider.StatusOpts) error {
	switch statusopts.Conclusion {
	case "skipped":
		statusopts.Conclusion = "STOPPED"
		statusopts.Title = "➖ Skipping this commit"
	case "neutral":
		statusopts.Conclusion = "STOPPED"
		statusopts.Title = "➖ CI has stopped"
	case "failure":
		statusopts.Conclusion = "FAILED"
		statusopts.Title = "❌ Failed"
	case "pending":
		statusopts.Conclusion = "INPROGRESS"
		statusopts.Title = "⚡ CI has started"
	case "success":
		statusopts.Conclusion = "SUCCESSFUL"
		statusopts.Title = "✅ Commit has been validated"
	case "completed":
		statusopts.Conclusion = "SUCCESSFUL"
		statusopts.Title = "✅ Completed"
	}
	detailsURL := event.Provider.URL
	if statusopts.DetailsURL != "" {
		detailsURL = statusopts.DetailsURL
	}

	cso := &bitbucket.CommitStatusOptions{
		Key:         pacopts.ApplicationName,
		Url:         detailsURL,
		State:       statusopts.Conclusion,
		Description: statusopts.Title,
	}
	cmo := &bitbucket.CommitsOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
		Revision: event.SHA,
	}

	if v.Client == nil {
		return fmt.Errorf("no token has been set, cannot set status")
	}

	_, err := v.Client.Repositories.Commits.CreateCommitStatus(cmo, cso)
	if err != nil {
		return err
	}
	if statusopts.Conclusion != "STOPPED" && statusopts.Status == "completed" &&
		statusopts.Text != "" && event.EventType == "pull_request" {
		prNumber, err := v.getPullRequestNumber(event.Event)
		if err != nil {
			return err
		}

		_, err = v.Client.Repositories.PullRequests.AddComment(
			&bitbucket.PullRequestCommentOptions{
				Owner:         event.Organization,
				RepoSlug:      event.Repository,
				PullRequestID: prNumber,
				Content: fmt.Sprintf("**%s** - %s\n\n%s", pacopts.ApplicationName,
					statusopts.Title, statusopts.Text),
			})
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path string) (string, error) {
	repoFileOpts := &bitbucket.RepositoryFilesOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
		Ref:      event.SHA,
		Path:     path,
	}

	repositoryFiles, err := v.Client.Repositories.Repository.ListFiles(repoFileOpts)
	if err != nil {
		return "", err
	}

	return v.concatAllYamlFiles(repositoryFiles, event)
}

func (v *Provider) GetFileInsideRepo(_ context.Context, runevent *info.Event, path string, targetBranch string) (string, error) {
	return v.getBlob(runevent, runevent.SHA, path)
}

func (v *Provider) SetClient(_ context.Context, event *info.Event) error {
	if event.Provider.User == "" {
		return fmt.Errorf("no git_provider.user has been set in the repo crd")
	}
	if event.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}
	v.Client = bitbucket.NewBasicAuth(event.Provider.User, event.Provider.Token)
	v.Token = &event.Provider.Token
	v.Username = &event.Provider.User
	return nil
}

func (v *Provider) GetCommitInfo(_ context.Context, event *info.Event) error {
	response, err := v.Client.Repositories.Commits.GetCommits(&bitbucket.CommitsOptions{
		Owner:       event.Organization,
		RepoSlug:    event.Repository,
		Branchortag: event.SHA,
	})
	if err != nil {
		return err
	}
	commitMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot convert")
	}
	values, ok := commitMap["values"].([]interface{})
	if !ok {
		return fmt.Errorf("cannot convert")
	}
	if len(values) == 0 {
		return fmt.Errorf("we did not get commit information from commit: %s", event.SHA)
	}
	commitinfo := &types.Commit{}
	err = mapstructure.Decode(values[0], commitinfo)
	if err != nil {
		return err
	}

	// Some silliness since we get first the account id and we fill it properly after
	event.SHATitle = commitinfo.Message
	event.SHAURL = commitinfo.Links.HTML.HRef
	event.SHA = commitinfo.Hash

	// now to get the default branch from repository.Get
	repo, err := v.Client.Repositories.Repository.Get(&bitbucket.RepositoryOptions{
		Owner:    event.Organization,
		RepoSlug: event.Repository,
	})
	if err != nil {
		return err
	}
	event.DefaultBranch = repo.Mainbranch.Name
	return nil
}

func (v *Provider) concatAllYamlFiles(objects []bitbucket.RepositoryFile, runevent *info.Event) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if strings.HasSuffix(value.Path, ".yaml") ||
			strings.HasSuffix(value.Path, ".yml") {
			data, err := v.getBlob(runevent, runevent.SHA, value.Path)
			if err != nil {
				return "", err
			}

			if allTemplates != "" && !strings.HasPrefix(data, "---") {
				allTemplates += "---"
			}
			allTemplates += "\n" + data + "\n"
		}
	}
	return allTemplates, nil
}

func (v *Provider) getBlob(runevent *info.Event, ref, path string) (string, error) {
	blob, err := v.Client.Repositories.Repository.GetFileBlob(&bitbucket.RepositoryBlobOptions{
		Owner:    runevent.Organization,
		RepoSlug: runevent.Repository,
		Ref:      ref,
		Path:     path,
	})
	if err != nil {
		return "", fmt.Errorf("cannot find %s on branch %s in repo %s/%s", path, ref, runevent.Organization, runevent.Repository)
	}
	return blob.String(), nil
}

func (v *Provider) getPullRequestNumber(eventPayload interface{}) (string, error) {
	prevent, ok := eventPayload.(*types.PullRequestEvent)
	if !ok {
		return "", fmt.Errorf("cannot convert event to PullRequestEvent")
	}
	prID := prevent.PullRequest.ID
	if prID == 0 {
		return "", fmt.Errorf("could not detect pull request ID")
	}
	return fmt.Sprintf("%d", prID), nil
}

func (v *Provider) Detect(reqHeader *http.Header, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, error) {
	isBitCloud := false
	event := reqHeader.Get("X-Event-Key")
	if event == "" {
		return false, false, logger, nil
	}

	eventInt, err := parsePayloadType(event, payload)
	if err != nil || eventInt == nil {
		return false, false, logger, err
	}

	// it is a Bitbucket cloud event
	isBitCloud = true

	setLoggerAndProceed := func(processEvent bool, err error) (bool, bool, *zap.SugaredLogger, error) {
		logger = logger.With("provider", "bitbucket-cloud", "event", reqHeader.Get("X-Request-Id"))
		return isBitCloud, processEvent, logger, err
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch e := eventInt.(type) {
	case *types.PullRequestEvent:
		if provider.Valid(event, []string{"pullrequest:created", "pullrequest:updated"}) {
			return setLoggerAndProceed(true, nil)
		}
		if provider.Valid(event, []string{"pullrequest:comment_created"}) {
			if provider.IsRetestComment(e.Comment.Content.Raw) {
				return setLoggerAndProceed(true, nil)
			}
			if provider.IsOkToTestComment(e.Comment.Content.Raw) {
				return setLoggerAndProceed(true, nil)
			}
		}
		return setLoggerAndProceed(false, nil)

	case *types.PushRequestEvent:
		if provider.Valid(event, []string{"repo:push"}) {
			if e.Push.Changes != nil {
				return setLoggerAndProceed(true, nil)
			}
		}
		return setLoggerAndProceed(false, nil)

	default:
		return setLoggerAndProceed(false, fmt.Errorf("bitbucket-cloud: event %s is not recognized", event))
	}
}
