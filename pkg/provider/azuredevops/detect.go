package azuredevops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/servicehooks"
	"go.uber.org/zap"
)

func (v *Provider) Detect(req *http.Request, payload string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	isADO := false
	eventType := req.Header.Get("X-Azure-DevOps-EventType")
	if eventType == "" {
		return false, false, logger, "no azure devops event", nil
	}

	// it is an Azure DevOps event
	isADO = true

	setLoggerAndProceed := func(processEvent bool, reason string, err error) (bool, bool, *zap.SugaredLogger,
		string, error,
	) {
		logger = logger.With("provider", "azure-devops", "event-id", req.Header.Get("X-Request-Id"))
		return isADO, processEvent, logger, reason, err
	}

	var event servicehooks.Event
	err := json.Unmarshal([]byte(payload), &event)
	if err != nil {
		return isADO, false, logger, "", err
	}

	//check if the event type provided in header and in the event json is same; is it necessary?
	// eventtype in comment json is ms.vss-code.git-pullrequest-comment-event so replaceing all - to . to match it with header
	normalizedEventType := strings.ReplaceAll(*event.EventType, "-", ".")
	if strings.Contains(normalizedEventType, eventType) {
		logger = logger.With("provider", "azuredevops", "event-type", eventType)
		switch eventType {
		case "git.push", "git.pullrequest.created", "git.pullrequest.updated", "git.pullrequest.comment":
			return setLoggerAndProceed(true, "", nil)
		default:
			return setLoggerAndProceed(false, fmt.Sprintf("Unsupported event type: %s", eventType), nil)
		}
	} else {
		return setLoggerAndProceed(false, fmt.Sprintf("event type in header %s and event json %s does not match", eventType, *event.EventType), nil)
	}
}
