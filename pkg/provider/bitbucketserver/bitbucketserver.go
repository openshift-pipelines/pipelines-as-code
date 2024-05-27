package bitbucketserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/google/go-github/v56/github"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

const taskStatusTemplate = `
{{range $taskrun := .TaskRunList }}* **{{ formatCondition $taskrun.PipelineRunTaskRunStatus.Status.Conditions }}**  {{ $taskrun.ConsoleLogURL }} *{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}*
{{ end }}`

var _ provider.Interface = (*Provider)(nil)

type Provider struct {
	Client                    *bbv1.APIClient
	Logger                    *zap.SugaredLogger
	run                       *params.Run
	baseURL                   string
	defaultBranchLatestCommit string
	pullRequestNumber         int
	apiURL                    string
	provenance                string
	projectKey                string
}

func (v *Provider) CheckPolicyAllowing(_ context.Context, _ *info.Event, _ []string) (bool, string) {
	return false, ""
}

// GetTaskURI TODO: Implement ME.
func (v *Provider) GetTaskURI(_ context.Context, _ *info.Event, _ string) (bool, string, error) {
	return false, "", nil
}

func (v *Provider) SetLogger(logger *zap.SugaredLogger) {
	v.Logger = logger
}

func (v *Provider) Validate(_ context.Context, _ *params.Run, event *info.Event) error {
	signature := event.Request.Header.Get("X-Hub-Signature")
	if event.Provider.WebhookSecret == "" && signature != "" {
		return fmt.Errorf("bitbucket-server failed validation: failed to find webhook secret")
	}
	return github.ValidateSignature(signature, event.Request.Payload, []byte(event.Provider.WebhookSecret))
}

// sanitizeTitle make sure we only get the tile by remove everything after \n.
func sanitizeTitle(s string) string {
	return strings.Split(s, "\n")[0]
}

func (v *Provider) CreateStatus(_ context.Context, event *info.Event, statusOpts provider.StatusOpts) error {
	detailsURL := event.Provider.URL
	switch statusOpts.Conclusion {
	case "skipped":
		statusOpts.Conclusion = "FAILED"
		statusOpts.Title = "➖ Skipping this commit"
	case "neutral":
		statusOpts.Conclusion = "FAILED"
		statusOpts.Title = "➖ CI has stopped"
	case "failure":
		statusOpts.Conclusion = "FAILED"
		statusOpts.Title = "❌ Failed"
	case "pending":
		statusOpts.Conclusion = "INPROGRESS"
		statusOpts.Title = "⚡ CI has started"
	case "success":
		statusOpts.Conclusion = "SUCCESSFUL"
		statusOpts.Title = "Commit has been validated"
	case "completed":
		statusOpts.Conclusion = "SUCCESSFUL"
		statusOpts.Title = "Completed"
	}
	if statusOpts.DetailsURL != "" {
		detailsURL = statusOpts.DetailsURL
	}
	if v.Client == nil {
		return fmt.Errorf("no token has been set, cannot set status")
	}

	key := statusOpts.PipelineRunName
	if key == "" {
		key = statusOpts.Conclusion
	}

	_, err := v.Client.DefaultApi.SetCommitStatus(
		event.SHA,
		bbv1.BuildStatus{
			State:       statusOpts.Conclusion,
			Name:        v.run.Info.Pac.ApplicationName,
			Key:         key,
			Description: statusOpts.Title,
			Url:         detailsURL,
		},
	)
	if err != nil {
		return err
	}

	onPr := ""
	if statusOpts.OriginalPipelineRunName != "" {
		onPr = "/" + statusOpts.OriginalPipelineRunName
	}
	bbcomment := bbv1.Comment{
		Text: fmt.Sprintf("**%s%s** - %s\n\n%s", v.run.Info.Pac.ApplicationName, onPr,
			statusOpts.Title, statusOpts.Text),
	}

	if statusOpts.Conclusion == "SUCCESSFUL" && statusOpts.Status == "completed" &&
		statusOpts.Text != "" && event.EventType == "pull_request" && v.pullRequestNumber > 0 {
		_, err := v.Client.DefaultApi.CreatePullRequestComment(
			v.projectKey, event.Repository, v.pullRequestNumber,
			bbcomment, []string{"application/json"})
		if err != nil {
			return err
		}
	}

	// TODO: Completed status
	return nil
}

func (v *Provider) concatAllYamlFiles(objects []string, runevent *info.Event) (string, error) {
	var allTemplates string
	for _, value := range objects {
		if strings.HasSuffix(value, ".yaml") ||
			strings.HasSuffix(value, ".yml") {
			data, err := v.getRaw(runevent, runevent.SHA, value)
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

func (v *Provider) getRaw(runevent *info.Event, revision, path string) (string, error) {
	localVarOptionals := map[string]interface{}{
		"at": revision,
	}
	resp, err := v.Client.DefaultApi.GetRawContent(v.projectKey, runevent.Repository, path, localVarOptionals)
	if err != nil {
		return "", err
	}
	return string(resp.Payload), nil
}

func (v *Provider) GetTektonDir(_ context.Context, event *info.Event, path, provenance string) (string, error) {
	v.provenance = provenance
	allValues, err := paginate(func(nextPage int) (*bbv1.APIResponse, error) {
		// according to the docs, if no at parameters is specified it will default to the default branch
		// cf: https://docs.atlassian.com/bitbucket-server/rest/4.1.0/bitbucket-rest.html#idp2425664
		localVarOptionals := map[string]interface{}{}
		if v.provenance == "source" {
			localVarOptionals = map[string]interface{}{"at": event.SHA}
			v.Logger.Infof("Using PipelineRun definition from source pull request SHA: %s", event.SHA)
		} else {
			v.Logger.Infof("Using PipelineRun definition from default_branch: %s", event.DefaultBranch)
		}
		if nextPage != 0 {
			localVarOptionals["start"] = nextPage
		}
		return v.Client.DefaultApi.StreamFiles_42(v.projectKey, event.Repository, path, localVarOptionals)
	})
	if err != nil {
		return "", err
	}

	fpathTmpl := []string{}
	for _, value := range allValues {
		vs, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("cannot get a string out of %s", value)
		}
		fpathTmpl = append(fpathTmpl, filepath.Join(path, vs))
	}

	return v.concatAllYamlFiles(fpathTmpl, event)
}

func (v *Provider) GetFileInsideRepo(_ context.Context, event *info.Event, path, targetBranch string) (string, error) {
	branch := event.SHA
	// TODO: this may be buggy? we need to figure out how to get the fromSource ref
	if targetBranch == event.DefaultBranch {
		branch = v.defaultBranchLatestCommit
	}

	ret, err := v.getRaw(event, branch, path)
	return ret, err
}

func (v *Provider) SetClient(ctx context.Context, run *params.Run, event *info.Event, _ *v1alpha1.Repository, _ *events.EventEmitter) error {
	if event.Provider.User == "" {
		return fmt.Errorf("no provider.user has been set in the repo crd")
	}
	if event.Provider.Token == "" {
		return fmt.Errorf("no provider.secret has been set in the repo crd")
	}
	if event.Provider.URL == "" {
		return fmt.Errorf("no provider.url has been set in the repo crd")
	}

	// make sure we have /rest at the end of the url
	if !strings.HasSuffix(event.Provider.URL, "/rest") {
		event.Provider.URL += "/rest"
	}

	// make sure we strip slashes from the end of the URL
	event.Provider.URL = strings.TrimSuffix(event.Provider.URL, "/")
	v.apiURL = event.Provider.URL

	basicAuth := bbv1.BasicAuth{UserName: event.Provider.User, Password: event.Provider.Token}

	ctx = context.WithValue(ctx, bbv1.ContextBasicAuth, basicAuth)
	cfg := bbv1.NewConfiguration(event.Provider.URL)
	v.Client = bbv1.NewAPIClient(ctx, cfg)
	v.run = run

	return nil
}

func (v *Provider) GetCommitInfo(_ context.Context, event *info.Event) error {
	localVarOptionals := map[string]interface{}{}
	resp, err := v.Client.DefaultApi.GetCommit(v.projectKey, event.Repository, event.SHA, localVarOptionals)
	if err != nil {
		return err
	}
	commitInfo := bbv1.Commit{}
	err = mapstructure.Decode(resp.Values, &commitInfo)
	if err != nil {
		return err
	}
	event.SHATitle = sanitizeTitle(commitInfo.Message)
	event.SHAURL = fmt.Sprintf("%s/projects/%s/repos/%s/commits/%s", v.baseURL, v.projectKey, event.Repository, event.SHA)

	resp, err = v.Client.DefaultApi.GetDefaultBranch(v.projectKey, event.Repository)
	if err != nil {
		return err
	}
	branchInfo := &bbv1.Branch{}
	err = mapstructure.Decode(resp.Values, branchInfo)
	if err != nil {
		return err
	}

	v.defaultBranchLatestCommit = branchInfo.LatestCommit
	event.DefaultBranch = branchInfo.DisplayID
	return nil
}

func (v *Provider) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{
		TaskStatusTMPL: taskStatusTemplate,
		Name:           "bitbucket-server",
	}
}

func (v *Provider) GetFiles(_ context.Context, _ *info.Event) (changedfiles.ChangedFiles, error) {
	return changedfiles.ChangedFiles{}, nil
}

func (v *Provider) CreateToken(_ context.Context, _ []string, _ *info.Event) (string, error) {
	return "", nil
}
