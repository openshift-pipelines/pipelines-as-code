package gitea

import (
	"encoding/json"
	"fmt"
	"net/http"

	giteastruct "code.gitea.io/gitea/modules/structs"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
)

// Detect processes event and detect if it is a gitea event, whether to process or reject it
// returns (if is a Gitea event, whether to process or reject, logger with event metadata,, error if any occurred)
func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	isGitea := false
	event := req.Header.Get("X-Gitea-Event-Type")
	if event == "" {
		return false, false, logger, "no gitea event", nil
	}

	isGitea = true
	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "gitea", "event-id", req.Header.Get("X-Request-Id"))
		return isGitea, processEvent, logger, reason, err
	}

	eventInt, err := parseWebhook(whEventType(event), []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}
	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *giteastruct.IssueCommentPayload:
		if gitEvent.Action == "created" &&
			gitEvent.Issue.PullRequest != nil &&
			gitEvent.Issue.State == "open" {
			if provider.IsTestRetestComment(gitEvent.Comment.Body) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsOkToTestComment(gitEvent.Comment.Body) {
				return setLoggerAndProceed(true, "", nil)
			}
			if provider.IsCancelComment(gitEvent.Comment.Body) {
				return setLoggerAndProceed(true, "", nil)
			}
			return setLoggerAndProceed(false, "", nil)
		}
		return setLoggerAndProceed(false, "not a issue comment we care about", nil)
	case *giteastruct.PullRequestPayload:
		if provider.Valid(string(gitEvent.Action), []string{"opened", "synchronize", "synchronized", "reopened"}) {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("not a merge event we care about: \"%s\"",
			string(gitEvent.Action)), nil)
	case *giteastruct.PushPayload:
		if gitEvent.Pusher != nil {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, "push: no pusher in event", nil)
	default:
		return setLoggerAndProceed(false, "", fmt.Errorf("gitea: event \"%s\" is not supported", event))
	}
}
