package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
)

const (
	apiPublicURL       = "https://gitlab.com"
	taskStatusTemplate = `
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
)

type Provider struct {
	Client            *gitlab.Client
	Token             *string
	targetProjectID   int
	sourceProjectID   int
	mergeRequestID    int
	userID            int
	pathWithNamespace string
	repoURL           string
}

func (v *Provider) Validate(ctx context.Context, params *params.Run, event *info.Event) error {
	return nil
}

// Detect processes event and detect if it is a gitlab event, whether to process or reject it
// returns (if is a GL event, whether to process or reject, logger with event metadata,, error if any occurred)
func (v *Provider) Detect(reqHeader *http.Header, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, error) {
	isGL := false
	event := reqHeader.Get("X-Gitlab-Event")
	if event == "" {
		return false, false, logger, nil
	}

	// it is a Gitlab event
	isGL = true

	setLoggerAndProceed := func() (bool, bool, *zap.SugaredLogger, error) {
		logger = logger.With("provider", "gitlab", "event", reqHeader.Get("X-Request-Id"))
		return isGL, true, logger, nil
	}

	eventInt, err := gitlab.ParseWebhook(gitlab.EventType(event), []byte(payload))
	if err != nil {
		return isGL, false, logger, err
	}
	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *gitlab.MergeEvent:
		return setLoggerAndProceed()
	case *gitlab.PushEvent:
		return setLoggerAndProceed()
	case *gitlab.MergeCommentEvent:
		if gitEvent.MergeRequest.State == "opened" {
			if provider.IsRetestComment(gitEvent.ObjectAttributes.Note) {
				return setLoggerAndProceed()
			}
			if provider.IsOkToTestComment(gitEvent.ObjectAttributes.Note) {
				return setLoggerAndProceed()
			}
		}
		return isGL, false, logger, nil
	default:
		return isGL, false, logger, fmt.Errorf("event %s is not supported", event)
	}
}

// If I understood properly, you can have "personal" projects and groups
// attached projects. But this doesn't seem to show in the API, so we
// are just doing it the path_with_namespace to get the "org".
//
// Note that "orgs/groups" may have subgroups, so we get the first parts
// as Orgs and the last element as Repo It's just a detail to show for
// UI, we actually don't use this field for access or other logical
// stuff.
func getOrgRepo(pathWithNamespace string) (string, string) {
	org := filepath.Dir(pathWithNamespace)
	org = strings.ReplaceAll(org, "/", "-")
	return org, filepath.Base(pathWithNamespace)
}

func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
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

		v.mergeRequestID = gitEvent.ObjectAttributes.IID
		v.targetProjectID = gitEvent.Project.ID
		v.sourceProjectID = gitEvent.ObjectAttributes.SourceProjectID
		v.userID = gitEvent.User.ID

		v.pathWithNamespace = gitEvent.ObjectAttributes.Target.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		processedEvent.TriggerTarget = "pull_request"
	case *gitlab.PushEvent:
		if len(gitEvent.Commits) == 0 {
			return nil, fmt.Errorf("no commits attached to this push event")
		}
		processedEvent = info.NewEvent()
		processedEvent.Sender = gitEvent.UserUsername
		processedEvent.DefaultBranch = gitEvent.Project.DefaultBranch
		processedEvent.URL = gitEvent.Project.WebURL
		processedEvent.SHA = gitEvent.Commits[0].ID
		processedEvent.SHAURL = gitEvent.Commits[0].URL
		processedEvent.SHATitle = gitEvent.Commits[0].Title
		processedEvent.HeadBranch = gitEvent.Ref
		processedEvent.BaseBranch = gitEvent.Ref
		processedEvent.TriggerTarget = "push"
		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		v.targetProjectID = gitEvent.ProjectID
		v.sourceProjectID = gitEvent.ProjectID
		v.userID = gitEvent.UserID
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

		v.pathWithNamespace = gitEvent.Project.PathWithNamespace
		processedEvent.Organization, processedEvent.Repository = getOrgRepo(v.pathWithNamespace)
		processedEvent.TriggerTarget = "pull_request"

		v.mergeRequestID = gitEvent.MergeRequest.IID
		v.targetProjectID = gitEvent.MergeRequest.TargetProjectID
		v.sourceProjectID = gitEvent.MergeRequest.SourceProjectID
		v.userID = gitEvent.User.ID
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

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
	}
}

func (v *Provider) SetClient(ctx context.Context, event *info.Event) error {
	var err error
	if event.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}

	// Try to detect automatically theapi url if url is not coming from public
	// gitlab. Unless user has set a spec.provider.url in its repo crd
	apiURL := apiPublicURL
	if event.Provider.URL != "" {
		apiURL = event.Provider.URL
	} else if !strings.HasPrefix(v.repoURL, apiPublicURL) {
		apiURL = strings.ReplaceAll(v.repoURL, v.pathWithNamespace, "")
	}

	v.Client, err = gitlab.NewClient(event.Provider.Token, gitlab.WithBaseURL(apiURL))
	if err != nil {
		return err
	}
	v.Token = &event.Provider.Token
	return nil
}

func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, pacOpts *info.PacOpts, statusOpts provider.StatusOpts) error {
	var detailsURL string
	if v.Client == nil {
		return fmt.Errorf("no gitlab client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	switch statusOpts.Conclusion {
	case "skipped":
		statusOpts.Conclusion = "canceled"
		statusOpts.Title = "skipped validating this commit"
	case "neutral":
		statusOpts.Conclusion = "canceled"
		statusOpts.Title = "stopped"
	case "failure":
		statusOpts.Conclusion = "failed"
		statusOpts.Title = "failed"
	case "success":
		statusOpts.Conclusion = "success"
		statusOpts.Title = "successfully validated your commit"
	case "completed":
		statusOpts.Conclusion = "success"
		statusOpts.Title = "completed"
	case "pending":
		statusOpts.Conclusion = "running"
	}
	if statusOpts.DetailsURL != "" {
		detailsURL = statusOpts.DetailsURL
	}

	body := fmt.Sprintf("**%s** has %s\n\n%s\n\n<small>Full log available [here](%s)</small>",
		pacOpts.ApplicationName, statusOpts.Title, statusOpts.Text, detailsURL)

	// in case we have access set the commit status, typically on MR from
	// another users we won't have it but it would work on push or MR from a
	// branch on the same repo or if token somehow can have access by other
	// means.
	// if we have an error fallback to send a issue comment
	opt := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(statusOpts.Conclusion),
		Name:        gitlab.String(pacOpts.ApplicationName),
		TargetURL:   gitlab.String(detailsURL),
		Description: gitlab.String(statusOpts.Title),
	}
	// nolint: dogsled
	_, _, _ = v.Client.Commits.SetCommitStatus(v.sourceProjectID, event.SHA, opt)
	if statusOpts.Conclusion != "running" {
		opt := &gitlab.CreateMergeRequestNoteOptions{Body: gitlab.String(body)}
		_, _, err := v.Client.Notes.CreateMergeRequestNote(v.targetProjectID, v.mergeRequestID, opt)
		return err
	}

	return nil
}

func (v *Provider) GetTektonDir(ctx context.Context, event *info.Event, path string) (string, error) {
	if v.Client == nil {
		return "", fmt.Errorf("no gitlab client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	opt := &gitlab.ListTreeOptions{
		Path: gitlab.String(path),
		Ref:  gitlab.String(event.HeadBranch),
	}

	objects, resp, err := v.Client.Repositories.ListTree(v.sourceProjectID, opt)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to list %s dir: %w", path, err)
	}

	return v.concatAllYamlFiles(objects, event)
}

// concatAllYamlFiles concat all yaml files from a directory as one big multi document yaml string
func (v *Provider) concatAllYamlFiles(objects []*gitlab.TreeNode, runevent *info.Event) (string, error) {
	var allTemplates string

	for _, value := range objects {
		if strings.HasSuffix(value.Name, ".yaml") ||
			strings.HasSuffix(value.Name, ".yml") {
			data, err := v.getObject(value.Path, runevent.HeadBranch, v.sourceProjectID)
			if err != nil {
				return "", err
			}
			if allTemplates != "" && !strings.HasPrefix(string(data), "---") {
				allTemplates += "---"
			}
			allTemplates += "\n" + string(data) + "\n"
		}
	}

	return allTemplates, nil
}

func (v *Provider) getObject(fname, branch string, pid int) ([]byte, error) {
	opt := &gitlab.GetRawFileOptions{
		Ref: gitlab.String(branch),
	}
	file, resp, err := v.Client.RepositoryFiles.GetRawFile(pid, fname, opt)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to get filename from api %s dir: %w", fname, err)
	}
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return []byte{}, nil
	}
	return file, nil
}

func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, _ string) (string, error) {
	getobj, err := v.getObject(path, runevent.HeadBranch, v.sourceProjectID)
	if err != nil {
		return "", err
	}
	return string(getobj), nil
}

func (v *Provider) GetCommitInfo(ctx context.Context, event *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no gitlab client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	return nil
}
