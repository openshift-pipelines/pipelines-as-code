package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v50/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretName = "pipelines-as-code-secret"
)

func GetAppIDAndPrivateKey(ctx context.Context, ns string, kube kubernetes.Interface) (int64, []byte, error) {
	secret, err := kube.CoreV1().Secrets(ns).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return 0, []byte{}, err
	}

	appID := secret.Data[keys.GithubApplicationID]
	applicationID, err := strconv.ParseInt(strings.TrimSpace(string(appID)), 10, 64)
	if err != nil {
		return 0, []byte{}, fmt.Errorf("could not parse the github application_id number from secret: %w", err)
	}

	privateKey := secret.Data[keys.GithubPrivateKey]
	return applicationID, privateKey, nil
}

func (v *Provider) GetAppToken(ctx context.Context, kube kubernetes.Interface, gheURL string, installationID int64, ns string) (string, error) {
	applicationID, privateKey, err := GetAppIDAndPrivateKey(ctx, ns, kube)
	if err != nil {
		return "", err
	}
	v.ApplicationID = &applicationID
	tr := http.DefaultTransport

	itr, err := ghinstallation.New(tr, applicationID, installationID, privateKey)
	if err != nil {
		return "", err
	}
	itr.InstallationTokenOptions = &github.InstallationTokenOptions{
		RepositoryIDs: v.RepositoryIDs,
	}

	if gheURL != "" {
		if !strings.HasPrefix(gheURL, "https://") && !strings.HasPrefix(gheURL, "http://") {
			gheURL = "https://" + gheURL
		}
		v.Client, _ = github.NewEnterpriseClient(gheURL, "", &http.Client{Transport: itr})
		itr.BaseURL = strings.TrimSuffix(v.Client.BaseURL.String(), "/")
	} else {
		v.Client = github.NewClient(&http.Client{Transport: itr})
	}

	// This is a hack when we have auth and api disassociated
	reqTokenURL := os.Getenv("PAC_GIT_PROVIDER_TOKEN_APIURL")
	if reqTokenURL != "" {
		itr.BaseURL = reqTokenURL
	}

	// Get a token ASAP because we need it for setting private repos
	token, err := itr.Token(ctx)
	if err != nil {
		return "", err
	}
	v.Token = github.String(token)

	return token, err
}

func (v *Provider) parseEventType(request *http.Request, event *info.Event) error {
	event.EventType = request.Header.Get("X-GitHub-Event")
	if event.EventType == "" {
		return fmt.Errorf("failed to find event type in request header")
	}

	event.Provider.URL = request.Header.Get("X-GitHub-Enterprise-Host")

	if event.EventType == "push" {
		event.TriggerTarget = "push"
	} else {
		event.TriggerTarget = "pull_request"
	}

	return nil
}

func getInstallationIDFromPayload(payload string) int64 {
	var data map[string]interface{}
	_ = json.Unmarshal([]byte(payload), &data)

	i := github.Installation{}
	installData, ok := data["installation"]
	if ok {
		installation, _ := json.Marshal(installData)
		_ = json.Unmarshal(installation, &i)
		return *i.ID
	}
	return -1
}

// ParsePayload will parse the payload and return the event
// it generate the github app token targeting the installation id
// this pieces of code is a bit messy because we need first getting a token to
// before parsing the payload.
//
// We need to get the token at first because in some case when coming from pull request
// comment (or recheck from the UI) we will use that token to get
// information about the PR that is not part of the payload.
//
// We then regenerate a second time the token scoped to the repo where the
// payload come from so we can avoid the scenario where an admin install the
// app on a github org which has a mixed of private and public repos and some of
// the public users should not have access to the private repos.
//
// Another thing: The payload is protected by the webhook signature so it cannot be tempered but even tho if it's
// tempered with and somehow a malicious user found the token and set their own github endpoint to hijack and
// exfiltrate the token, it would fail since the jwt token generation will fail, so we are safe here.
// a bit too far fetched but i don't see any way we can exploit this.
func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	event := info.NewEvent()
	// TODO: we should not have getenv in code only in main
	systemNS := os.Getenv("SYSTEM_NAMESPACE")
	if err := v.parseEventType(request, event); err != nil {
		return nil, err
	}

	installationIDFrompayload := getInstallationIDFromPayload(payload)
	if installationIDFrompayload != -1 {
		var err error
		// TODO: move this out of here when we move al config inside context
		if event.Provider.Token, err = v.GetAppToken(ctx, run.Clients.Kube, event.Provider.URL, installationIDFrompayload, systemNS); err != nil {
			return nil, err
		}
	}

	eventInt, err := github.ParseWebHook(event.EventType, []byte(payload))
	if err != nil {
		return nil, err
	}

	// should not get invalid json since we already check it in github.ParseWebHook
	_ = json.Unmarshal([]byte(payload), &eventInt)

	processedEvent, err := v.processEvent(ctx, event, eventInt)
	if err != nil {
		return nil, err
	}

	processedEvent.InstallationID = installationIDFrompayload
	processedEvent.GHEURL = event.Provider.URL
	processedEvent.Provider.URL = event.Provider.URL

	// regenerate token scoped to the repo IDs
	if run.Info.Pac.SecretGHAppRepoScoped && installationIDFrompayload != -1 && len(v.RepositoryIDs) > 0 {
		repoLists := []string{}
		if run.Info.Pac.SecretGhAppTokenScopedExtraRepos != "" {
			// this is going to show up a lot in the logs but i guess that
			// would make people fix the value instead of being lost into
			// the top of the logs at controller start.
			for _, configValue := range strings.Split(run.Info.Pac.SecretGhAppTokenScopedExtraRepos, ",") {
				configValueS := strings.TrimSpace(configValue)
				if configValueS == "" {
					continue
				}
				repoLists = append(repoLists, configValueS)
			}
			v.Logger.Infof("Github token scope extended to %v keeping SecretGHAppRepoScoped to true", repoLists)
		}
		token, err := v.CreateToken(ctx, repoLists, run, processedEvent)
		if err != nil {
			return nil, err
		}
		processedEvent.Provider.Token = token
	}

	return processedEvent, nil
}

func (v *Provider) processEvent(ctx context.Context, event *info.Event, eventInt interface{}) (*info.Event, error) {
	var processedEvent *info.Event
	var err error

	processedEvent = info.NewEvent()
	processedEvent.Event = eventInt

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("check run rerequest is only supported with github apps integration")
		}

		if *gitEvent.Action != "rerequested" {
			return nil, fmt.Errorf("only issue recheck is supported in checkrunevent")
		}
		return v.handleReRequestEvent(ctx, gitEvent)
	case *github.CheckSuiteEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("check suite rerequest is only supported with github apps integration")
		}

		if *gitEvent.Action != "rerequested" {
			return nil, fmt.Errorf("only issue recheck is supported in checkrunevent")
		}
		return v.handleCheckSuites(ctx, gitEvent)
	case *github.IssueCommentEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("gitops style comments operation is only supported with github apps integration")
		}
		processedEvent, err = v.handleIssueCommentEvent(ctx, gitEvent)
		if err != nil {
			return nil, err
		}
	case *github.PushEvent:
		processedEvent.Organization = gitEvent.GetRepo().GetOwner().GetLogin()
		processedEvent.Repository = gitEvent.GetRepo().GetName()
		processedEvent.DefaultBranch = gitEvent.GetRepo().GetDefaultBranch()
		processedEvent.URL = gitEvent.GetRepo().GetHTMLURL()
		v.RepositoryIDs = []int64{gitEvent.GetRepo().GetID()}
		processedEvent.SHA = gitEvent.GetHeadCommit().GetID()
		// on push event we may not get a head commit but only
		if processedEvent.SHA == "" {
			processedEvent.SHA = gitEvent.GetBefore()
		}
		processedEvent.SHAURL = gitEvent.GetHeadCommit().GetURL()
		processedEvent.SHATitle = gitEvent.GetHeadCommit().GetMessage()
		processedEvent.Sender = gitEvent.GetSender().GetLogin()
		processedEvent.BaseBranch = gitEvent.GetRef()
		processedEvent.EventType = event.TriggerTarget
		processedEvent.HeadBranch = processedEvent.BaseBranch // in push events Head Branch is the same as Basebranch
		processedEvent.BaseURL = gitEvent.GetRepo().GetHTMLURL()
		processedEvent.HeadURL = processedEvent.BaseURL // in push events Head URL is the same as BaseURL
	case *github.PullRequestEvent:
		processedEvent = info.NewEvent()
		processedEvent.Repository = gitEvent.GetRepo().GetName()
		processedEvent.Organization = gitEvent.GetRepo().Owner.GetLogin()
		processedEvent.DefaultBranch = gitEvent.GetRepo().GetDefaultBranch()
		processedEvent.SHA = gitEvent.GetPullRequest().Head.GetSHA()
		processedEvent.URL = gitEvent.GetRepo().GetHTMLURL()
		processedEvent.BaseBranch = gitEvent.GetPullRequest().Base.GetRef()
		processedEvent.HeadBranch = gitEvent.GetPullRequest().Head.GetRef()
		processedEvent.BaseURL = gitEvent.GetPullRequest().Base.GetRepo().GetHTMLURL()
		processedEvent.HeadURL = gitEvent.GetPullRequest().Head.GetRepo().GetHTMLURL()
		processedEvent.Sender = gitEvent.GetPullRequest().GetUser().GetLogin()
		processedEvent.EventType = event.EventType
		processedEvent.PullRequestNumber = gitEvent.GetPullRequest().GetNumber()
		// getting the repository ids of the base and head of the pull request
		// to scope the token to
		v.RepositoryIDs = []int64{
			gitEvent.GetPullRequest().GetBase().GetRepo().GetID(),
		}
	default:
		return nil, errors.New("this event is not supported")
	}

	processedEvent.TriggerTarget = event.TriggerTarget
	processedEvent.Provider.Token = event.Provider.Token

	return processedEvent, nil
}

func (v *Provider) handleReRequestEvent(ctx context.Context, event *github.CheckRunEvent) (*info.Event, error) {
	runevent := info.NewEvent()
	runevent.Organization = event.GetRepo().GetOwner().GetLogin()
	runevent.Repository = event.GetRepo().GetName()
	runevent.URL = event.GetRepo().GetHTMLURL()
	runevent.DefaultBranch = event.GetRepo().GetDefaultBranch()
	runevent.SHA = event.GetCheckRun().GetCheckSuite().GetHeadSHA()
	runevent.HeadBranch = event.GetCheckRun().GetCheckSuite().GetHeadBranch()
	runevent.HeadURL = event.GetCheckRun().GetCheckSuite().GetRepository().GetHTMLURL()
	// If we don't have a pull_request in this it probably mean a push
	if len(event.GetCheckRun().GetCheckSuite().PullRequests) == 0 {
		runevent.BaseBranch = runevent.HeadBranch
		runevent.BaseURL = runevent.HeadURL
		runevent.EventType = "push"
		// we allow the rerequest user here, not the push user, i guess it's
		// fine because you can't do a rereq without being a github owner?
		runevent.Sender = event.GetSender().GetLogin()
		return runevent, nil
	}
	runevent.PullRequestNumber = event.GetCheckRun().GetCheckSuite().PullRequests[0].GetNumber()
	runevent.TriggerTarget = "pull_request"
	v.Logger.Infof("Recheck of PR %s/%s#%d has been requested", runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	return v.getPullRequest(ctx, runevent)
}

func (v *Provider) handleCheckSuites(ctx context.Context, event *github.CheckSuiteEvent) (*info.Event, error) {
	runevent := info.NewEvent()
	runevent.Organization = event.GetRepo().GetOwner().GetLogin()
	runevent.Repository = event.GetRepo().GetName()
	runevent.URL = event.GetRepo().GetHTMLURL()
	runevent.DefaultBranch = event.GetRepo().GetDefaultBranch()
	runevent.SHA = event.GetCheckSuite().GetHeadSHA()
	runevent.HeadBranch = event.GetCheckSuite().GetHeadBranch()
	runevent.HeadURL = event.GetCheckSuite().GetRepository().GetHTMLURL()
	// If we don't have a pull_request in this it probably mean a push
	// we are not able to know which
	if len(event.GetCheckSuite().PullRequests) == 0 {
		runevent.BaseBranch = runevent.HeadBranch
		runevent.BaseURL = runevent.HeadURL
		runevent.EventType = "push"
		runevent.TriggerTarget = "push"
		// we allow the rerequest user here, not the push user, i guess it's
		// fine because you can't do a rereq without being a github owner?
		runevent.Sender = event.GetSender().GetLogin()
		return runevent, nil
		// return nil, fmt.Errorf("check suite event is not supported for push events")
	}
	runevent.PullRequestNumber = event.GetCheckSuite().PullRequests[0].GetNumber()
	runevent.TriggerTarget = "pull_request"
	v.Logger.Infof("Rerun of all check on PR %s/%s#%d has been requested", runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	return v.getPullRequest(ctx, runevent)
}

func convertPullRequestURLtoNumber(pullRequest string) (int, error) {
	prNumber, err := strconv.Atoi(path.Base(pullRequest))
	if err != nil {
		return -1, fmt.Errorf("bad pull request number html_url number: %w", err)
	}
	return prNumber, nil
}

func (v *Provider) handleIssueCommentEvent(ctx context.Context, event *github.IssueCommentEvent) (*info.Event, error) {
	action := "recheck"
	runevent := info.NewEvent()
	runevent.Organization = event.GetRepo().GetOwner().GetLogin()
	runevent.Repository = event.GetRepo().GetName()
	runevent.Sender = event.GetSender().GetLogin()
	// Always set the trigger target as pull_request on issue comment events
	runevent.TriggerTarget = "pull_request"
	if !event.GetIssue().IsPullRequest() {
		return info.NewEvent(), fmt.Errorf("issue comment is not coming from a pull_request")
	}

	// if it is a /test or /retest comment with pipelinerun name figure out the pipelinerun name
	if provider.IsTestRetestComment(event.GetComment().GetBody()) {
		runevent.TargetTestPipelineRun = provider.GetPipelineRunFromTestComment(event.GetComment().GetBody())
	}
	if provider.IsCancelComment(event.GetComment().GetBody()) {
		action = "cancellation"
		runevent.CancelPipelineRuns = true
		runevent.TargetCancelPipelineRun = provider.GetPipelineRunFromCancelComment(event.GetComment().GetBody())
	}
	// We are getting the full URL so we have to get the last part to get the PR number,
	// we don't have to care about URL query string/hash and other stuff because
	// that comes up from the API.
	var err error
	runevent.PullRequestNumber, err = convertPullRequestURLtoNumber(event.GetIssue().GetPullRequestLinks().GetHTMLURL())
	if err != nil {
		return info.NewEvent(), err
	}

	v.Logger.Infof("issue_comment: pipelinerun %s on %s/%s#%d has been requested", action, runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	return v.getPullRequest(ctx, runevent)
}
