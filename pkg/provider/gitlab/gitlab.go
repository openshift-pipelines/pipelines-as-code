package gitlab

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
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
<td>{{ formatCondition $taskrun.PipelineRunTaskRunStatus.Status.Conditions }}</td>
<td>{{ formatDuration $taskrun.PipelineRunTaskRunStatus.Status.StartTime $taskrun.PipelineRunTaskRunStatus.Status.CompletionTime }}</td><td>

{{ $taskrun.ConsoleLogURL }}

</td></tr>
{{- end }}
</table>`
)

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	Client            *gitlab.Client
	Logger            *zap.SugaredLogger
	Token             *string
	targetProjectID   int
	sourceProjectID   int
	userID            int
	pathWithNamespace string
	repoURL           string
	apiURL            string
}

// GetTaskURI TODO: Implement me
func (v *Provider) GetTaskURI(_ context.Context, _ *params.Run, _ *info.Event, _ string) (bool, string, error) {
	return false, "", nil
}

// CheckPolicyAllowing TODO: Implement ME
func (v *Provider) CheckPolicyAllowing(_ context.Context, _ *info.Event, _ []string) (bool, string) {
	return false, ""
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(_ context.Context, _ *params.Run, event *info.Event) error {
	token := event.Request.Header.Get("X-Gitlab-Token")
	if event.Provider.WebhookSecret == "" && token != "" {
		return fmt.Errorf("gitlab failed validaton: failed to find webhook secret")
	}

	if subtle.ConstantTimeCompare([]byte(event.Provider.WebhookSecret), []byte(token)) == 0 {
		return fmt.Errorf("gitlab failed validaton: event's secret doesn't match with webhook secret")
	}
	return nil
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

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		APIURL:         apiPublicURL,
		Name:           "gitlab",
	}
}

func (v *Provider) SetClient(_ context.Context, _ *params.Run, runevent *info.Event, _ *v1alpha1.Settings) error {
	var err error
	if runevent.Provider.Token == "" {
		return fmt.Errorf("no git_provider.secret has been set in the repo crd")
	}

	// Try to detect automatically the API url if url is not coming from public
	// gitlab. Unless user has set a spec.provider.url in its repo crd
	apiURL := ""
	switch {
	case runevent.Provider.URL != "":
		apiURL = runevent.Provider.URL
	case v.repoURL != "" && !strings.HasPrefix(v.repoURL, apiPublicURL):
		apiURL = strings.ReplaceAll(v.repoURL, v.pathWithNamespace, "")
	case runevent.URL != "":
		burl, err := url.Parse(runevent.URL)
		if err != nil {
			return err
		}
		apiURL = fmt.Sprintf("%s://%s", burl.Scheme, burl.Host)
	default:
		// this really should not happen but let's just hope this is it
		apiURL = apiPublicURL
	}
	v.apiURL = apiURL

	v.Client, err = gitlab.NewClient(runevent.Provider.Token, gitlab.WithBaseURL(apiURL))
	if err != nil {
		return err
	}
	v.Token = &runevent.Provider.Token

	// if we don't have sourceProjectID (ie: incoming-webhook) then try to set
	// it ASAP if we can.
	if v.sourceProjectID == 0 && runevent.Organization != "" && runevent.Repository != "" {
		projectSlug := filepath.Join(runevent.Organization, runevent.Repository)
		projectinfo, _, err := v.Client.Projects.GetProject(projectSlug, &gitlab.GetProjectOptions{})
		if err != nil {
			return err
		}
		// TODO: we really need to move out the runevent.*ProjecTID to v.*ProjectID,
		// I just spent half an hour debugging because i didn't realise it was there instead in v.*
		v.sourceProjectID = projectinfo.ID
		runevent.SourceProjectID = projectinfo.ID
		runevent.TargetProjectID = projectinfo.ID
		runevent.DefaultBranch = projectinfo.DefaultBranch
	}

	return nil
}

func (v *Provider) CreateStatus(_ context.Context, _ versioned.Interface, event *info.Event, pacOpts *info.PacOpts,
	statusOpts provider.StatusOpts,
) error {
	var detailsURL string
	if v.Client == nil {
		return fmt.Errorf("no gitlab client has been initialized, " +
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

	onPr := ""
	if statusOpts.OriginalPipelineRunName != "" {
		onPr = "/" + statusOpts.OriginalPipelineRunName
	}
	body := fmt.Sprintf("**%s%s** has %s\n\n%s\n\n<small>Full log available [here](%s)</small>",
		pacOpts.ApplicationName, onPr, statusOpts.Title, statusOpts.Text, detailsURL)

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
	//nolint: dogsled
	_, _, _ = v.Client.Commits.SetCommitStatus(event.SourceProjectID, event.SHA, opt)

	// only add a note when we are on a MR
	if event.EventType == "pull_request" || event.EventType == "Merge_Request" || event.EventType == "Merge Request" {
		mopt := &gitlab.CreateMergeRequestNoteOptions{Body: gitlab.String(body)}
		_, _, err := v.Client.Notes.CreateMergeRequestNote(event.TargetProjectID, event.PullRequestNumber, mopt)
		return err
	}
	return nil
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path, provenance string) (string, error) {
	if v.Client == nil {
		return "", fmt.Errorf("no gitlab client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	// default set provenance from head
	revision := event.HeadBranch
	if provenance == "default_branch" {
		revision = event.DefaultBranch
		v.Logger.Infof("Using PipelineRun definition from default_branch: %s", event.DefaultBranch)
	} else {
		v.Logger.Infof("Using PipelineRun definition from source merge request SHA: %s", event.SHA)
	}

	opt := &gitlab.ListTreeOptions{
		Path:      gitlab.String(path),
		Ref:       gitlab.String(revision),
		Recursive: gitlab.Bool(true),
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

func (v *Provider) GetFileInsideRepo(_ context.Context, runevent *info.Event, path, _ string) (string, error) {
	getobj, err := v.getObject(path, runevent.HeadBranch, v.sourceProjectID)
	if err != nil {
		return "", err
	}
	return string(getobj), nil
}

func (v *Provider) GetCommitInfo(_ context.Context, runevent *info.Event) error {
	if v.Client == nil {
		return fmt.Errorf("no gitlab client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}

	// if we don't have a SHA (ie: incoming-webhook) then get it from the branch
	// and populate in the runevent.
	if runevent.SHA == "" && runevent.HeadBranch != "" {
		branchinfo, _, err := v.Client.Commits.GetCommit(v.sourceProjectID, runevent.HeadBranch)
		if err != nil {
			return err
		}
		runevent.SHA = branchinfo.ID
		runevent.SHATitle = branchinfo.Title
		runevent.SHAURL = branchinfo.WebURL
	}

	return nil
}

func (v *Provider) GetFiles(_ context.Context, runevent *info.Event) ([]string, error) {
	if v.Client == nil {
		return []string{}, fmt.Errorf("no gitlab client has been initialized, " +
			"exiting... (hint: did you forget setting a secret on your repo?)")
	}
	if runevent.TriggerTarget == "pull_request" {
		mrchanges, _, err := v.Client.MergeRequests.GetMergeRequestChanges(v.sourceProjectID, runevent.PullRequestNumber, &gitlab.GetMergeRequestChangesOptions{})
		if err != nil {
			return []string{}, err
		}

		result := []string{}
		for _, change := range mrchanges.Changes {
			result = append(result, change.NewPath)
		}
		return result, nil
	}

	if runevent.TriggerTarget == "push" {
		pushChanges, _, err := v.Client.Commits.GetCommitDiff(v.sourceProjectID, runevent.SHA, &gitlab.GetCommitDiffOptions{})
		if err != nil {
			return []string{}, err
		}
		result := []string{}
		for _, change := range pushChanges {
			result = append(result, change.NewPath)
		}
		return result, nil
	}
	return []string{}, nil
}

func (v *Provider) CreateToken(_ context.Context, _ []string, _ *params.Run, _ *info.Event) (string, error) {
	return "", nil
}
