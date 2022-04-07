package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v42/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"go.uber.org/zap"
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

func (v *Provider) getAppToken(ctx context.Context, info *info.PacOpts) error {
	installationIDEnv := os.Getenv("PAC_INSTALLATION_ID")
	workspacePath := os.Getenv("PAC_WORKSPACE_SECRET")

	if installationIDEnv == "" || workspacePath == "" {
		return nil
	}

	installationID, err := strconv.ParseInt(installationIDEnv, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse installation_id: %w", err)
	}
	v.ApplicationID = &installationID

	// check if the path exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace path %s or env PAC_WORKSPACE_SECRET does not exist", workspacePath)
	}

	// read github-application-id from the secret workspace
	b, err := ioutil.ReadFile(filepath.Join(workspacePath, "github-application-id"))
	if err != nil {
		return fmt.Errorf("could not open the github-application-id key in secret: %w", err)
	}
	applicationID, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse the github application_id number from secret: %w", err)
	}

	// read private_key from the secret workspace
	privatekey := filepath.Join(workspacePath, "github-private-key")
	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, applicationID, installationID, privatekey)
	if err != nil {
		return err
	}

	// getting the baseurl from go-github since it has all the logic in there
	gheURL := os.Getenv("PAC_GIT_PROVIDER_APIURL")
	if gheURL != "" {
		if !strings.HasPrefix(gheURL, "https://") {
			gheURL = "https://" + gheURL
		}
		v.Client, _ = github.NewEnterpriseClient(gheURL, "", &http.Client{Transport: itr})
		itr.BaseURL = strings.TrimSuffix(v.Client.BaseURL.String(), "/")
	} else {
		v.Client = github.NewClient(&http.Client{Transport: itr})
	}

	// This is a hack when we have auth and api dissascoiated
	reqTokenURL := os.Getenv("PAC_GIT_PROVIDER_TOKEN_APIURL")
	if reqTokenURL != "" {
		itr.BaseURL = reqTokenURL
	}

	// Get a token ASAP because we need it for setting private repos
	token, err := itr.Token(ctx)
	if err != nil {
		return err
	}
	v.Token = github.String(token)
	info.ProviderToken = token

	return err
}

// ParsePayload parse payload event
// TODO: this piece of code is just plain silly
func (v *Provider) ParsePayload(ctx context.Context, run *params.Run, payload string) (*info.Event, error) {
	var processedevent *info.Event

	// get the app token if it exist first
	if err := v.getAppToken(ctx, run.Info.Pac); err != nil {
		return nil, err
	}

	payloadTreated := v.payloadFix(payload)
	event, err := github.ParseWebHook(run.Info.Event.EventType, payloadTreated)
	if err != nil {
		return nil, err
	}

	// should not get invalid json since we already check it in github.ParseWebHook
	_ = json.Unmarshal(payloadTreated, &event)

	switch event := event.(type) {
	case *github.CheckRunEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("reqrequest is only supported with github apps integration")
		}

		if run.Info.Event.TriggerTarget != "issue-recheck" {
			return nil, fmt.Errorf("only issue recheck is supported in checkrunevent")
		}
		processedevent, err = v.handleReRequestEvent(ctx, run.Clients.Log, event)
		if err != nil {
			return nil, err
		}
	case *github.IssueCommentEvent:
		if v.Client == nil {
			return nil, fmt.Errorf("gitops style comments operation is only supported with github apps integration")
		}
		processedevent, err = v.handleIssueCommentEvent(ctx, run.Clients.Log, event)
		if err != nil {
			return nil, err
		}
	case *github.PushEvent:
		processedevent = &info.Event{
			Organization:  event.GetRepo().GetOwner().GetLogin(),
			Repository:    event.GetRepo().GetName(),
			DefaultBranch: event.GetRepo().GetDefaultBranch(),
			URL:           event.GetRepo().GetHTMLURL(),
			SHA:           event.GetHeadCommit().GetID(),
			SHAURL:        event.GetHeadCommit().GetURL(),
			SHATitle:      event.GetHeadCommit().GetMessage(),
			Sender:        event.GetSender().GetLogin(),
			BaseBranch:    event.GetRef(),
			EventType:     run.Info.Event.TriggerTarget,
		}

		processedevent.HeadBranch = processedevent.BaseBranch // in push events Head Branch is the same as Basebranch
	case *github.PullRequestEvent:
		processedevent = &info.Event{
			Organization:  event.GetRepo().Owner.GetLogin(),
			Repository:    event.GetRepo().GetName(),
			DefaultBranch: event.GetRepo().GetDefaultBranch(),
			SHA:           event.GetPullRequest().Head.GetSHA(),
			URL:           event.GetRepo().GetHTMLURL(),
			BaseBranch:    event.GetPullRequest().Base.GetRef(),
			HeadBranch:    event.GetPullRequest().Head.GetRef(),
			Sender:        event.GetPullRequest().GetUser().GetLogin(),
			EventType:     run.Info.Event.EventType,
		}

	default:
		return nil, errors.New("this event is not supported")
	}

	processedevent.Event = event
	processedevent.TriggerTarget = run.Info.Event.TriggerTarget

	return processedevent, nil
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
