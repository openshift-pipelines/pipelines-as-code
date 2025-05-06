package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

// Detect detects events and validates if it is a valid gitlab event Pipelines as Code supports and
// decides whether to process or reject it.
// returns a boolean value whether to process or reject, logger with event metadata, and error if any occurred.
func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	isGL := false
	event := req.Header.Get("X-Gitlab-Event")
	if event == "" {
		return false, false, logger, "no gitlab event", nil
	}

	// it is a GitLab event
	isGL = true

	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "gitlab", "event-id", req.Header.Get("X-Request-Id"))
		return isGL, processEvent, logger, reason, err
	}

	eventInt, err := gitlab.ParseWebhook(gitlab.EventType(event), []byte(payload))
	if err != nil {
		return setLoggerAndProceed(false, "", err)
	}
	_ = json.Unmarshal([]byte(payload), &eventInt)

	switch gitEvent := eventInt.(type) {
	case *gitlab.MergeEvent:
		if provider.Valid(gitEvent.ObjectAttributes.Action, []string{"open", "reopen", "update"}) {
			return setLoggerAndProceed(true, "", nil)
		}

		// on a MR Update only react when there is Oldrev set, since this means
		// there is a Push of commit in there
		if gitEvent.ObjectAttributes.Action == "update" && gitEvent.ObjectAttributes.OldRev != "" {
			return setLoggerAndProceed(true, "", nil)
		}
		if provider.Valid(gitEvent.ObjectAttributes.Action, []string{"open", "reopen", "close"}) {
			return setLoggerAndProceed(true, "", nil)
		}

		return setLoggerAndProceed(false, fmt.Sprintf("not a merge event we care about: \"%s\"", gitEvent.ObjectAttributes.Action), nil)
	case *gitlab.PushEvent, *gitlab.TagEvent:
		return setLoggerAndProceed(true, "", nil)
	case *gitlab.MergeCommentEvent:
		if gitEvent.MergeRequest.State == "opened" {
			return setLoggerAndProceed(true, "", nil)
		}
		return setLoggerAndProceed(false, "comments on closed merge requests is not supported", nil)
	case *gitlab.CommitCommentEvent:
		comment := gitEvent.ObjectAttributes.Note
		if gitEvent.ObjectAttributes.Action == gitlab.CommentEventActionCreate {
			if provider.IsTestRetestComment(comment) || provider.IsCancelComment(comment) {
				return setLoggerAndProceed(true, "", nil)
			}
			// truncate comment to make logs readable
			if len(comment) > 50 {
				comment = comment[:50] + "..."
			}
			return setLoggerAndProceed(false, fmt.Sprintf("gitlab: commit_comment: unsupported GitOps comment \"%s\" on pushed commits", comment), nil)
		}
		return setLoggerAndProceed(false, fmt.Sprintf("gitlab: commit_comment: unsupported action \"%s\" with comment \"%s\"", gitEvent.ObjectAttributes.Action, comment), nil)
	default:
		return setLoggerAndProceed(false, "", fmt.Errorf("gitlab: event \"%s\" is not supported", event))
	}
}
