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
)

const (
	apiPublicURL       = "https://gitlab.com/api/v4"
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
	Client          *gitlab.Client
	Token           *string
	targetProjectID int
	sourceProjectID int
	mergeRequestID  int
	userID          int
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

func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, payload string) (*info.Event, error) {
	var processedevent *info.Event

	payloadB := []byte(payload)
	event, err := gitlab.ParseWebhook(gitlab.EventType(run.Info.Event.EventType), payloadB)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(payloadB, &event)

	switch event := event.(type) {
	case *gitlab.MergeEvent:
		processedevent = &info.Event{
			// Organization:  event.GetRepo().GetOwner().GetLogin(),
			Sender:        event.User.Username,
			DefaultBranch: event.Project.DefaultBranch,
			URL:           event.Project.WebURL,
			SHA:           event.ObjectAttributes.LastCommit.ID,
			SHAURL:        event.ObjectAttributes.LastCommit.URL,
			SHATitle:      event.ObjectAttributes.Title,
			HeadBranch:    event.ObjectAttributes.SourceBranch,
			BaseBranch:    event.ObjectAttributes.TargetBranch,
		}

		v.mergeRequestID = event.ObjectAttributes.IID
		v.targetProjectID = event.Project.ID
		v.sourceProjectID = event.ObjectAttributes.SourceProjectID
		v.userID = event.User.ID

		processedevent.Organization, processedevent.Repository = getOrgRepo(event.ObjectAttributes.Target.PathWithNamespace)
		processedevent.TriggerTarget = "pull_request"
	case *gitlab.PushEvent:
		processedevent = &info.Event{
			Sender:        event.UserUsername,
			DefaultBranch: event.Project.DefaultBranch,
			URL:           event.Project.WebURL,
			SHA:           event.Commits[0].ID,
			SHAURL:        event.Commits[0].URL,
			SHATitle:      event.Commits[0].Title,
			HeadBranch:    event.Ref,
			BaseBranch:    event.Ref,
		}
		processedevent.TriggerTarget = "push"
		processedevent.Organization, processedevent.Repository = getOrgRepo(event.Project.PathWithNamespace)
		v.targetProjectID = event.ProjectID
		v.sourceProjectID = event.ProjectID
		v.userID = event.UserID
	case *gitlab.MergeCommentEvent:
		processedevent = &info.Event{
			Sender:        event.User.Username,
			DefaultBranch: event.Project.DefaultBranch,
			URL:           event.Project.WebURL,
			SHA:           event.MergeRequest.LastCommit.ID,
			SHAURL:        event.MergeRequest.LastCommit.URL,
			// TODO: change this back to Title when we get this pr available merged https://github.com/xanzy/go-gitlab/pull/1406/files
			SHATitle:   event.MergeRequest.LastCommit.Message,
			BaseBranch: event.MergeRequest.TargetBranch,
			HeadBranch: event.MergeRequest.SourceBranch,
		}
		processedevent.Organization, processedevent.Repository = getOrgRepo(event.Project.PathWithNamespace)
		processedevent.TriggerTarget = "pull_request"

		v.mergeRequestID = event.MergeRequest.IID
		v.targetProjectID = event.MergeRequest.TargetProjectID
		v.sourceProjectID = event.MergeRequest.SourceProjectID
		v.userID = event.User.ID
	default:
		return nil, fmt.Errorf("event %s is not supported", run.Info.Event.EventType)
	}

	processedevent.Event = event

	// Remove the " Hook" suffix so looks better in status, and since we don't
	// really use it anymore we good to do whatever we want with it for
	// cosmetics.
	processedevent.EventType = strings.ReplaceAll(run.Info.Event.EventType, " Hook", "")
	return processedevent, nil
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
	}
}

func (v *Provider) SetClient(ctx context.Context, opts *info.PacOpts) error {
	var err error
	if opts.ProviderToken == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}
	v.Client, err = gitlab.NewClient(opts.ProviderToken, gitlab.WithBaseURL(apiPublicURL))
	if err != nil {
		return err
	}
	v.Token = &opts.ProviderToken
	return nil
}

func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, pacOpts *info.PacOpts, statusOpts provider.StatusOpts) error {
	var detailsURL string
	if v.Client == nil {
		return fmt.Errorf("no gitlab client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	// TODO: not supported it on free, we will need an account on ultimate to be able to do it :\
	if statusOpts.Status == "in_progress" {
		return nil
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
	}
	if statusOpts.DetailsURL != "" {
		detailsURL = statusOpts.DetailsURL
	}

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
	_, _, toIgnoreErr := v.Client.Commits.SetCommitStatus(v.sourceProjectID, event.SHA, opt)

	if toIgnoreErr != nil && event.TriggerTarget == "pull_request" {
		opt := &gitlab.CreateMergeRequestNoteOptions{
			Body: gitlab.String(
				fmt.Sprintf("**%s** has %s\n\n%s\n\n<small>Full log available [here](%s)</small>", pacOpts.ApplicationName,
					statusOpts.Title, statusOpts.Text, detailsURL)),
		}
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
	if err != nil {
		return "", fmt.Errorf("failed to list %s dir: %w", path, err)
	}
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return "", nil
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

func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	getobj, err := v.getObject(path, runevent.HeadBranch, v.sourceProjectID)
	if err != nil {
		return "", err
	}

	if getobj == nil {
		return "", fmt.Errorf("cannot find %s in this repository", path)
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
