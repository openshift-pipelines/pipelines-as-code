package github

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/v56/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

// Detect processes event and detect if it is a github event, whether to process or reject it
// returns (if is a GH event, whether to process or reject, error if any occurred)
func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	// gitea set x-github-event too, so skip it for the gitea driver
	if h := req.Header.Get("X-Gitea-Event-Type"); h != "" {
		return false, false, logger, "", nil
	}
	isGH := false
	eventType := req.Header.Get("X-Github-Event")
	if eventType == "" {
		return false, false, logger, "", nil
	}

	// it is a Github event
	isGH = true

	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "github", "event-id", req.Header.Get("X-GitHub-Delivery"))
		return isGH, processEvent, logger, reason, err
	}

	eventInt, err := github.ParseWebHook(eventType, []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)
	eType, errReason := detectTriggerTypeFromPayload(eventType, eventInt)
	if eType != "" {
		return setLoggerAndProceed(true, "", nil)
	}

	return setLoggerAndProceed(false, errReason, nil)
}

// detectTriggerTypeFromPayload will detect the event type from the payload,
// filtering out the events that are not supported.
// first arg will get the event type and the second one will get an error string explaining why it's not supported.
func detectTriggerTypeFromPayload(ghEventType string, eventInt any) (info.TriggerType, string) {
	switch event := eventInt.(type) {
	case *github.PushEvent:
		if event.GetPusher() != nil {
			return info.TriggerTypePush, ""
		}
		return "", "no pusher in payload"
	case *github.PullRequestEvent:
		if provider.Valid(event.GetAction(), []string{"opened", "synchronize", "synchronized", "reopened"}) {
			return info.TriggerTypePullRequest, ""
		}
		return "", fmt.Sprintf("pull_request: unsupported action \"%s\"", event.GetAction())
	case *github.IssueCommentEvent:
		if event.GetAction() == "created" &&
			event.GetIssue().IsPullRequest() &&
			event.GetIssue().GetState() == "open" {
			if provider.IsTestRetestComment(event.GetComment().GetBody()) {
				return info.TriggerTypeRetest, ""
			}
			if provider.IsOkToTestComment(event.GetComment().GetBody()) {
				return info.TriggerTypeOkToTest, ""
			}
			if provider.IsCancelComment(event.GetComment().GetBody()) {
				return info.TriggerTypeCancel, ""
			}
		}
		return "", "comment: not a PAC gitops pull request comment"
	case *github.CheckSuiteEvent:
		if event.GetAction() == "rerequested" && event.GetCheckSuite() != nil {
			return info.TriggerTypeCheckSuiteRerequested, ""
		}
		return "", fmt.Sprintf("check_suite: unsupported action \"%s\"", event.GetAction())
	case *github.CheckRunEvent:
		if event.GetAction() == "rerequested" && event.GetCheckRun() != nil {
			return info.TriggerTypeCheckRunRerequested, ""
		}
		return "", fmt.Sprintf("check_run: unsupported action \"%s\"", event.GetAction())
	case *github.CommitCommentEvent:
		if event.GetAction() == "created" {
			if provider.IsTestRetestComment(event.GetComment().GetBody()) {
				return info.TriggerTypeRetest, ""
			}
			if provider.IsOkToTestComment(event.GetComment().GetBody()) {
				return info.TriggerTypeOkToTest, ""
			}
			if provider.IsCancelComment(event.GetComment().GetBody()) {
				return info.TriggerTypeCancel, ""
			}
		}
		return "", fmt.Sprintf("commit_comment: unsupported action \"%s\"", event.GetAction())
	}
	return "", fmt.Sprintf("github: event \"%v\" is not supported", ghEventType)
}
