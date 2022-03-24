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
	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretName = "pipelines-as-code-secret"
)

// payloadFix since we are getting a bunch of \r\n or \n and others from triggers/github, so let just
// workaround it. Originally from https://stackoverflow.com/a/52600147
func (v *Provider) payloadFix(payload string) []byte {
	replacement := " "
	replacer := strings.NewReplacer(
		"\r\n", replacement,
		"\r", replacement,
		"\n", replacement,
		"\v", replacement,
		"\f", replacement,
		"\u0085", replacement,
		"\u2028", replacement,
		"\u2029", replacement,
	)
	return []byte(replacer.Replace(payload))
}

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

	privateKey := secret.Data["github-private-key"]

	tr := http.DefaultTransport

	itr, err := ghinstallation.New(tr, applicationID, installationID, privateKey)
	if err != nil {
		return "", err
	}

	if gheURL != "" {
		if !strings.HasPrefix(gheURL, "https://") {
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

	event.ProviderURL = request.Header.Get("X-GitHub-Enterprise-Host")

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
	event := &info.Event{}

	if err := v.parseEventType(request, event); err != nil {
		return nil, err
	}

	id := getInstallationIDFromPayload(payload)

	if id != -1 {
		// get the app token if it exist first
		var err error
		event.ProviderToken, err = v.getAppToken(ctx, run.Clients.Kube, event.ProviderURL, id)
		if err != nil {
			return nil, err
		}
	}

	payloadTreated := v.payloadFix(payload)
	eventInt, err := github.ParseWebHook(event.EventType, payloadTreated)
	if err != nil {
		return nil, err
	}

	// should not get invalid json since we already check it in github.ParseWebHook
	_ = json.Unmarshal(payloadTreated, &eventInt)

	processedEvent, err := v.processEvent(ctx, run, event, eventInt)
	if err != nil {
		return nil, err
	}

	return processedEvent, nil
}

func (v *Provider) processEvent(ctx context.Context, run *params.Run, event *info.Event, eventInt interface{}) (*info.Event, error) {
	var processedEvent *info.Event
	var err error

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("reqrequest is only supported with github apps integration")
		}

		if *gitEvent.Action != "rerequested" {
			return nil, fmt.Errorf("only issue recheck is supported in checkrunevent")
		}
		processedEvent, err = v.handleReRequestEvent(ctx, run.Clients.Log, gitEvent)
		if err != nil {
			return nil, err
		}
	case *github.IssueCommentEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("gitops style comments operation is only supported with github apps integration")
		}
		processedEvent, err = v.handleIssueCommentEvent(ctx, run.Clients.Log, gitEvent)
		if err != nil {
			return nil, err
		}

	case *github.PushEvent:
		processedEvent = &info.Event{
			Organization:  gitEvent.GetRepo().GetOwner().GetLogin(),
			Repository:    gitEvent.GetRepo().GetName(),
			DefaultBranch: gitEvent.GetRepo().GetDefaultBranch(),
			URL:           gitEvent.GetRepo().GetHTMLURL(),
			SHA:           gitEvent.GetHeadCommit().GetID(),
			SHAURL:        gitEvent.GetHeadCommit().GetURL(),
			SHATitle:      gitEvent.GetHeadCommit().GetMessage(),
			Sender:        gitEvent.GetSender().GetLogin(),
			BaseBranch:    gitEvent.GetRef(),
			EventType:     event.TriggerTarget,
		}

		processedEvent.HeadBranch = processedEvent.BaseBranch // in push events Head Branch is the same as Basebranch
	case *github.PullRequestEvent:
		processedEvent = &info.Event{
			Organization:  gitEvent.GetRepo().Owner.GetLogin(),
			Repository:    gitEvent.GetRepo().GetName(),
			DefaultBranch: gitEvent.GetRepo().GetDefaultBranch(),
			SHA:           gitEvent.GetPullRequest().Head.GetSHA(),
			URL:           gitEvent.GetRepo().GetHTMLURL(),
			BaseBranch:    gitEvent.GetPullRequest().Base.GetRef(),
			HeadBranch:    gitEvent.GetPullRequest().Head.GetRef(),
			Sender:        gitEvent.GetPullRequest().GetUser().GetLogin(),
			EventType:     event.EventType,
		}

	default:
		return nil, errors.New("this event is not supported")
	}

	processedEvent.Event = eventInt
	processedEvent.TriggerTarget = event.TriggerTarget
	processedEvent.ProviderToken = event.ProviderToken

	return processedEvent, nil
}

func (v *Provider) handleReRequestEvent(ctx context.Context, log *zap.SugaredLogger, event *github.CheckRunEvent) (*info.Event, error) {
	runevent := &info.Event{
		Organization:  event.GetRepo().GetOwner().GetLogin(),
		Repository:    event.GetRepo().GetName(),
		URL:           event.GetRepo().GetHTMLURL(),
		DefaultBranch: event.GetRepo().GetDefaultBranch(),
		SHA:           event.GetCheckRun().GetCheckSuite().GetHeadSHA(),
		HeadBranch:    event.GetCheckRun().GetCheckSuite().GetHeadBranch(),
	}

	// If we don't have a pull_request in this it probably mean a push
	if len(event.GetCheckRun().GetCheckSuite().PullRequests) == 0 {
		runevent.BaseBranch = runevent.HeadBranch
		runevent.EventType = "push"
		// we allow the rerequest user here, not the push user, i guess it's
		// fine because you can't do a rereq without being a github owner?
		runevent.Sender = event.GetSender().GetLogin()
		return runevent, nil
	}
	prNumber := event.GetCheckRun().GetCheckSuite().PullRequests[0].GetNumber()
	log.Infof("Recheck of PR %s/%s#%d has been requested", runevent.Organization, runevent.Repository, prNumber)
	return v.getPullRequest(ctx, runevent, prNumber)
}

func convertPullRequestURLtoNumber(pullRequest string) (int, error) {
	prNumber, err := strconv.Atoi(path.Base(pullRequest))
	if err != nil {
		return -1, fmt.Errorf("bad pull request number html_url number: %w", err)
	}
	return prNumber, nil
}

func (v *Provider) handleIssueCommentEvent(ctx context.Context, log *zap.SugaredLogger, event *github.IssueCommentEvent) (*info.Event, error) {
	runevent := &info.Event{
		Organization: event.GetRepo().GetOwner().GetLogin(),
		Repository:   event.GetRepo().GetName(),
		Sender:       event.GetSender().GetLogin(),
		// Always set the trigger target as pull_request on issue comment events
		TriggerTarget: "pull_request",
	}
	if !event.GetIssue().IsPullRequest() {
		return &info.Event{}, fmt.Errorf("issue comment is not coming from a pull_request")
	}

	// We are getting the full URL so we have to get the last part to get the PR number,
	// we don't have to care about URL query string/hash and other stuff because
	// that comes up from the API.
	prNumber, err := convertPullRequestURLtoNumber(event.GetIssue().GetPullRequestLinks().GetHTMLURL())
	if err != nil {
		return &info.Event{}, err
	}

	log.Infof("PR recheck from issue commment on %s/%s#%d has been requested", runevent.Organization, runevent.Repository, prNumber)
	return v.getPullRequest(ctx, runevent, prNumber)
}
