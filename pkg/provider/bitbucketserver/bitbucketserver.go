package bitbucketserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/mitchellh/mapstructure"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/types"
	"go.uber.org/zap"
)

const taskStatusTemplate = `
{{range $taskrun := .TaskRunList }}* **{{ formatCondition $taskrun.Status.Conditions }}**  {{ $taskrun.ConsoleLogURL }} *{{ formatDuration $taskrun.Status.StartTime $taskrun.Status.CompletionTime }}* 
{{ end }}`

type Provider struct {
	Client                    *bbv1.APIClient
	baseURL                   string
	defaultBranchLatestCommit string
	pullRequestNumber         int
	apiURL                    string
	projectKey                string
}

// func (v *Provider) ParseEventType(request *http.Request, event *info.Event) error {
//	panic("implement me")
// }

// sanitizeTitle make sure we only get the tile by remove everything after \n.
func sanitizeTitle(s string) string {
	return strings.Split(s, "\n")[0]
}

func (v *Provider) CreateStatus(ctx context.Context, event *info.Event, pacOpts *info.PacOpts, statusOpts provider.StatusOpts) error {
	detailsURL := event.ProviderURL
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
			Name:        pacOpts.ApplicationName,
			Key:         key,
			Description: statusOpts.Title,
			Url:         detailsURL,
		},
	)
	if err != nil {
		return err
	}

	bbcomment := bbv1.Comment{
		Text: fmt.Sprintf("**%s** - %s\n\n%s", pacOpts.ApplicationName,
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

func (v *Provider) getRaw(runevent *info.Event, revision string, path string) (string, error) {
	localVarOptionals := map[string]interface{}{
		"at": revision,
	}
	resp, err := v.Client.DefaultApi.GetContent_11(v.projectKey, runevent.Repository, path, localVarOptionals)
	if err != nil {
		return "", err
	}
	return string(resp.Payload), nil
}

func (v *Provider) GetTektonDir(ctx context.Context, event *info.Event, path string) (string, error) {
	allValues, err := paginate(func(nextPage int) (*bbv1.APIResponse, error) {
		localVarOptionals := map[string]interface{}{"at": event.SHA}
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

func (v *Provider) GetFileInsideRepo(ctx context.Context, event *info.Event, path string, targetBranch string) (string, error) {
	branch := event.SHA
	// TODO: this may be buggy? we need to figure out how to get the fromSource ref
	if targetBranch == event.DefaultBranch {
		branch = v.defaultBranchLatestCommit
	}

	ret, err := v.getRaw(event, branch, path)
	return ret, err
}

func (v *Provider) SetClient(ctx context.Context, event *info.Event) error {
	if event.ProviderUser == "" {
		return fmt.Errorf("no provider.user has been set in the repo crd")
	}
	if event.ProviderToken == "" {
		return fmt.Errorf("no provider.secret has been set in the repo crd")
	}
	if event.ProviderURL == "" {
		return fmt.Errorf("no provider.url has been set in the repo crd")
	}

	// make sure we have /rest at the end of the url
	if !strings.HasSuffix(event.ProviderURL, "/rest") {
		event.ProviderURL += "/rest"
	}

	// make sure we strip slashes from the end of the URL
	event.ProviderURL = strings.TrimSuffix(event.ProviderURL, "/")
	v.apiURL = event.ProviderURL

	basicAuth := bbv1.BasicAuth{UserName: event.ProviderUser, Password: event.ProviderToken}

	ctx = context.WithValue(ctx, bbv1.ContextBasicAuth, basicAuth)
	cfg := bbv1.NewConfiguration(event.ProviderURL)
	v.Client = bbv1.NewAPIClient(ctx, cfg)

	return nil
}

func (v *Provider) GetCommitInfo(ctx context.Context, event *info.Event) error {
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
	}
}

// Detect processes event and detect if it is a bitbucket server event, whether to process or reject it
// returns (if is a bitbucket server event, whether to process or reject, error if any occurred)
func (v *Provider) Detect(reqHeader *http.Header, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, error) {
	isBitServer := false
	event := reqHeader.Get("X-Event-Key")
	if event == "" {
		return false, false, logger, nil
	}

	eventPayload, err := parsePayloadType(event)
	if err != nil || eventPayload == nil {
		return false, false, logger, err
	}

	// it is a Bitbucket server event
	isBitServer = true

	setLoggerAndProceed := func() (bool, bool, *zap.SugaredLogger, error) {
		logger = logger.With("provider", "bitbucket-server", "event", reqHeader.Get("X-Request-Id"))
		return isBitServer, true, logger, nil
	}

	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return isBitServer, false, logger, err
	}

	switch e := eventPayload.(type) {
	case *types.PullRequestEvent:
		if provider.Valid(event, []string{"pr:from_ref_updated", "pr:opened"}) {
			return setLoggerAndProceed()
		}
		if provider.Valid(event, []string{"pr:comment:added", "pr:comment:edited"}) {
			if matches, _ := regexp.MatchString(provider.RetestRegex, e.Comment.Text); matches {
				return setLoggerAndProceed()
			}
			if matches, _ := regexp.MatchString(provider.OktotestRegex, e.Comment.Text); matches {
				return setLoggerAndProceed()
			}
		}
		return isBitServer, false, logger, nil

	case *types.PushRequestEvent:
		if provider.Valid(event, []string{"repo:refs_changed"}) {
			if e.Changes != nil {
				return setLoggerAndProceed()
			}
		}
		return isBitServer, false, logger, nil

	default:
		return isBitServer, false, logger, fmt.Errorf("event %s is not supported", event)
	}
}
