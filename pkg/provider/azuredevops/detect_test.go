package azuredevops

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/servicehooks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestProvider_Detect(t *testing.T) {
	gitPush := "git.push"
	gitPRCreated := "git.pullrequest.created"
	gitPRUpdated := "git.pullrequest.updated"

	tests := []struct {
		name          string
		wantErr       bool
		wantErrString string
		isADO         bool
		processReq    bool
		event         servicehooks.Event
		eventType     string
		wantReason    string
	}{
		{
			name:       "no event type",
			eventType:  "",
			isADO:      false,
			processReq: false,
			wantReason: "no azure devops event",
		},
		{
			name:       "unsupported event type",
			eventType:  "build.completed",
			isADO:      true,
			processReq: false,
			wantReason: "Unsupported event type: build.completed",
		},
		{
			name: "git push event",
			event: servicehooks.Event{
				EventType: &gitPush,
			},
			eventType:  "git.push",
			isADO:      true,
			processReq: true,
		},
		{
			name: "pull request created event",
			event: servicehooks.Event{
				EventType: &gitPRCreated,
			},
			eventType:  "git.pullrequest.created",
			isADO:      true,
			processReq: true,
		},
		{
			name: "pull request updated event",
			event: servicehooks.Event{
				EventType: &gitPRUpdated,
			},
			eventType:  "git.pullrequest.updated",
			isADO:      true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t).Sugar()
			payload, err := json.Marshal(tt.event)
			assert.NoError(t, err)

			header := http.Header{}
			header.Set("X-Azure-DevOps-EventType", tt.eventType)
			req := &http.Request{Header: header}

			v := Provider{}
			isADO, processReq, _, reason, err := v.Detect(req, string(payload), logger)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrString != "" {
					assert.Contains(t, err.Error(), tt.wantErrString)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.isADO, isADO)
			assert.Equal(t, tt.processReq, processReq)
			if tt.wantReason != "" {
				assert.True(t, strings.Contains(reason, tt.wantReason), "Reason should contain the expected message")
			}
		})
	}
}
