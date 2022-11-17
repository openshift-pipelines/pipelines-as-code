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
	"github.com/google/go-github/v47/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretName = "pipelines-as-code-secret"
)

func (v *Provider) getAppToken(ctx context.Context, kube kubernetes.Interface, gheURL string, installationID int64) (string, error) {
	// TODO: move this out of here
	ns := os.Getenv("SYSTEM_NAMESPACE")
	secret, err := kube.CoreV1().Secrets(ns).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return "", err
	}

	appID := secret.Data["github-application-id"]
	applicationID, err := strconv.ParseInt(strings.TrimSpace(string(appID)), 10, 64)
	if err != nil {
		return "", fmt.Errorf("could not parse the github application_id number from secret: %w", err)
	}
	v.ApplicationID = &applicationID

	privateKey := secret.Data["github-private-key"]

	tr := http.DefaultTransport

	itr, err := ghinstallation.New(tr, applicationID, installationID, privateKey)
	if err != nil {
		return "", err
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

func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	event := info.NewEvent()

	if err := v.parseEventType(request, event); err != nil {
		return nil, err
	}

	id := getInstallationIDFromPayload(payload)

	if id != -1 {
		// get the app token if it exist first
		var err error
		event.Provider.Token, err = v.getAppToken(ctx, run.Clients.Kube, event.Provider.URL, id)
		if err != nil {
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

	processedEvent.InstallationID = id
	processedEvent.GHEURL = event.Provider.URL

	return processedEvent, nil
}

func (v *Provider) processEvent(ctx context.Context, event *info.Event, eventInt interface{}) (*info.Event, error) {
	var processedEvent *info.Event
	var err error

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("check run rerequest is only supported with github apps integration")
		}

		if *gitEvent.Action != "rerequested" {
			return nil, fmt.Errorf("only issue recheck is supported in checkrunevent")
		}
		processedEvent, err = v.handleReRequestEvent(ctx, gitEvent)
		if err != nil {
			return nil, err
		}
	case *github.IssueCommentEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("gitops style comments operation is only supported with github apps integration")
		}
		processedEvent, err = v.handleIssueCommentEvent(ctx, gitEvent)
		if err != nil {
			return nil, err
		}

	case *github.PushEvent:
		processedEvent = info.NewEvent()
		processedEvent.Organization = gitEvent.GetRepo().GetOwner().GetLogin()
		processedEvent.Repository = gitEvent.GetRepo().GetName()
		processedEvent.DefaultBranch = gitEvent.GetRepo().GetDefaultBranch()
		processedEvent.URL = gitEvent.GetRepo().GetHTMLURL()
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
	case *github.PullRequestEvent:
		processedEvent = info.NewEvent()
		processedEvent.Repository = gitEvent.GetRepo().GetName()
		processedEvent.Organization = gitEvent.GetRepo().Owner.GetLogin()
		processedEvent.DefaultBranch = gitEvent.GetRepo().GetDefaultBranch()
		processedEvent.SHA = gitEvent.GetPullRequest().Head.GetSHA()
		processedEvent.URL = gitEvent.GetRepo().GetHTMLURL()
		processedEvent.BaseBranch = gitEvent.GetPullRequest().Base.GetRef()
		processedEvent.HeadBranch = gitEvent.GetPullRequest().Head.GetRef()
		processedEvent.Sender = gitEvent.GetPullRequest().GetUser().GetLogin()
		processedEvent.EventType = event.EventType
		processedEvent.PullRequestNumber = gitEvent.GetPullRequest().GetNumber()
	default:
		return nil, errors.New("this event is not supported")
	}

	processedEvent.Event = eventInt
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
	// If we don't have a pull_request in this it probably mean a push
	if len(event.GetCheckRun().GetCheckSuite().PullRequests) == 0 {
		runevent.BaseBranch = runevent.HeadBranch
		runevent.EventType = "push"
		// we allow the rerequest user here, not the push user, i guess it's
		// fine because you can't do a rereq without being a github owner?
		runevent.Sender = event.GetSender().GetLogin()
		return runevent, nil
	}
	runevent.PullRequestNumber = event.GetCheckRun().GetCheckSuite().PullRequests[0].GetNumber()
	v.Logger.Infof("Recheck of PR %s/%s#%d has been requested", runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
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
		runevent.TargetTestPipelineRun = provider.GetPipelineRunFromComment(event.GetComment().GetBody())
	}

	// We are getting the full URL so we have to get the last part to get the PR number,
	// we don't have to care about URL query string/hash and other stuff because
	// that comes up from the API.
	var err error
	runevent.PullRequestNumber, err = convertPullRequestURLtoNumber(event.GetIssue().GetPullRequestLinks().GetHTMLURL())
	if err != nil {
		return info.NewEvent(), err
	}

	v.Logger.Infof("PR recheck from issue commment on %s/%s#%d has been requested", runevent.Organization, runevent.Repository, runevent.PullRequestNumber)
	return v.getPullRequest(ctx, runevent)
}
