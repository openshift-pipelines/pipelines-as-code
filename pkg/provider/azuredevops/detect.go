package azuredevops

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	logger = logger.With("provider", "azuredevops", "event-type", event.EventType)

	// Simplified switch, expand as needed based on the Azure DevOps events you handle
	switch eventType {
	case "git.push", "git.pullrequest.created", "git.pullrequest.updated":
		return setLoggerAndProceed(true, "", nil)
	default:
		return setLoggerAndProceed(false, fmt.Sprintf("Unsupported event type: %s", eventType), nil)
	}
}
