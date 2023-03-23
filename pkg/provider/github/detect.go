package github

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/v50/github"
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
	event := req.Header.Get("X-Github-Event")
	if event == "" {
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

	eventInt, err := github.ParseWebHook(event, []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}

	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *github.CheckRunEvent:
		if gitEvent.GetAction() == "rerequested" && gitEvent.GetCheckRun() != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("check_run: unsupported action \"%s\"", gitEvent.GetAction()), nil)

	case *github.IssueCommentEvent:
		if gitEvent.GetAction() == "created" &&
			gitEvent.GetIssue().IsPullRequest() &&
			gitEvent.GetIssue().GetState() == "open" {
			if provider.IsTestRetestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsOkToTestComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsCancelComment(gitEvent.GetComment().GetBody()) {
				return setLoggerAndProceed(true, "", nil)
			}
			return setLoggerAndProceed(false, "", nil)
		}
		return setLoggerAndProceed(false, "issue: not a gitops pull request comment", nil)
	case *github.PushEvent:
		if gitEvent.GetPusher() != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, "push: no pusher in event", nil)

	case *github.PullRequestEvent:
		if provider.Valid(gitEvent.GetAction(), []string{"opened", "synchronize", "synchronized", "reopened"}) {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("pull_request: unsupported action \"%s\"", gitEvent.GetAction()), nil)

	default:
		return setLoggerAndProceed(false, fmt.Sprintf("github: event \"%v\" is not supported", event), nil)
	}
}
