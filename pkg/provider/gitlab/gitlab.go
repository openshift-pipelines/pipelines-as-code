package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
			Repository:    event.Repository.Name,
			Sender:        event.User.Username,
			DefaultBranch: event.Project.DefaultBranch,
			URL:           event.Project.GitHTTPURL,
			SHA:           event.ObjectAttributes.LastCommit.ID,
			SHAURL:        event.ObjectAttributes.LastCommit.URL,
			SHATitle:      event.ObjectAttributes.Title,
			EventType:     run.Info.Event.TriggerTarget,
			HeadBranch:    event.ObjectAttributes.SourceBranch,
			BaseBranch:    event.ObjectAttributes.TargetBranch,
		}

		v.targetProjectID = event.Project.ID
		v.sourceProjectID = event.ObjectAttributes.SourceProjectID

		// If I understood properly, you can have "personal" projects and org
		// attached projects. But this doesn't seem to show in the API, so I am
		// just with the path_with_namespace to get the "org". It's just a
		// wording/detail it doesn't matter
		splitted := strings.Split(event.ObjectAttributes.Target.PathWithNamespace, "/")
		processedevent.Organization = splitted[0]
	default:
		return nil, errors.New("this event is not supported")
	}

	processedevent.Event = event
	processedevent.TriggerTarget = run.Info.Event.TriggerTarget
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

// todo: move to common since we use this in others too
func getCheckName(status provider.StatusOpts, pacopts *info.PacOpts) string {
	if pacopts.ApplicationName != "" {
		if status.OriginalPipelineRunName == "" {
			return pacopts.ApplicationName
		}
		return fmt.Sprintf("%s / %s", pacopts.ApplicationName, status.OriginalPipelineRunName)
	}
	return status.OriginalPipelineRunName
}

func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, pacOpts *info.PacOpts, statusOpts provider.StatusOpts) error {
	if v.Client == nil {
		return fmt.Errorf("no github client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	if statusOpts.Status == "in_progress" {
		statusOpts.Title = "CI has Started"
		statusOpts.Summary = fmt.Sprintf("%s is running.", pacOpts.ApplicationName)
		statusOpts.Conclusion = "running"
	}

	opt := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(statusOpts.Conclusion),
		Name:        gitlab.String(getCheckName(statusOpts, pacOpts)),
		TargetURL:   gitlab.String(pacOpts.LogURL),
		Description: gitlab.String(statusOpts.Title),
	}
	_, _, err := v.Client.Commits.SetCommitStatus(v.targetProjectID, event.SHA, opt)
	return err
}

func (v *Provider) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	if v.Client == nil {
		return false, fmt.Errorf("no github client has been initiliazed, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	// TODO implement me
	return true, nil
}

func (v *Provider) GetTektonDir(ctx context.Context, event *info.Event, path string) (string, error) {
	if v.Client == nil {
		return "", fmt.Errorf("no github client has been initiliazed, " +
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
			data, err := v.getObject(value.Path, runevent)
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

func (v *Provider) getObject(fname string, runevent *info.Event) ([]byte, error) {
	opt := &gitlab.GetRawFileOptions{
		Ref: gitlab.String(runevent.HeadBranch),
	}
	file, resp, err := v.Client.RepositoryFiles.GetRawFile(v.sourceProjectID, fname, opt)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to get filename from api %s dir: %w", fname, err)
	}
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return []byte{}, nil
	}
	return file, nil
}

func (v *Provider) GetFileInsideRepo(ctx context.Context, runevent *info.Event, path, target string) (string, error) {
	getobj, err := v.getObject(path, runevent)
	if err != nil {
		return "", err
	}

	if getobj == nil {
		return "", fmt.Errorf("cannot find %s in this repository", path)
	}

	return string(getobj), nil
}

func (v *Provider) GetCommitInfo(ctx context.Context, event *info.Event) error {
	return nil
	// if v.Client == nil {
	// 	return fmt.Errorf("no github client has been initiliazed, " +
	// 		"exiting... (hint: did you forget setting a secret on your repo?)")
	// }
	// v.Client.Commits.GetCommit(v.projectID, event.SHA)
	// // TODO implement me
	// panic("implement me")
}
