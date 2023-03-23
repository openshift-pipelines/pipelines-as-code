package bitbucketcloud

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	bbcloudtest "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/test"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestParsePayload(t *testing.T) {
	tests := []struct {
		name                      string
		payloadEvent              interface{}
		wantErr                   bool
		expectedSender            string
		expectedAccountID         string
		expectedSHA               string
		eventType                 string
		sourceIP                  string
		allowedConfig             map[string]map[string]string
		additionalAllowedsourceIP string
		targetPipelinerun         string
		cancelPipelinerun         string
	}{
		{
			name:              "parse push request",
			payloadEvent:      bbcloudtest.MakePushEvent("PushAccountID", "Barbie", "slighlyhashed"),
			expectedSender:    "Barbie",
			expectedAccountID: "PushAccountID",
			expectedSHA:       "slighlyhashed",
			eventType:         "repo:push",
		},
		{
			name:              "parse pull request",
			payloadEvent:      bbcloudtest.MakePREvent("TheAccountID", "Sender", "SHABidou", ""),
			expectedAccountID: "TheAccountID",
			expectedSender:    "Sender",
			expectedSHA:       "SHABidou",
			eventType:         "pullrequest:created",
		},
		{
			name:              "check source ip allowed",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pullrequest:created",
			sourceIP:          "1.2.3.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "1.2.3.10/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:              "check source ip allowed multiple xff",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pullrequest:updated",
			sourceIP:          "127.0.0.1,30.30.30.30,1.2.3.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "1.2.3.10/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:              "check source ip not allowed",
			wantErr:           true,
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pullrequest:created",
			sourceIP:          "1.2.3.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "10.12.33.22/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:                      "additional source ip allowed",
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			expectedAccountID:         "account",
			expectedSender:            "sender",
			expectedSHA:               "sha",
			eventType:                 "pullrequest:created",
			sourceIP:                  "1.2.3.1",
			additionalAllowedsourceIP: "1.2.3.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "10.12.33.22/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:                      "additional network allowed with spaces",
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			expectedAccountID:         "account",
			expectedSender:            "sender",
			expectedSHA:               "sha",
			eventType:                 "pullrequest:created",
			sourceIP:                  "1.2.3.3",
			additionalAllowedsourceIP: "1.2.3.4, 1.2.3.0/16",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "10.12.33.22/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:                      "not allowed with additional ips",
			wantErr:                   true,
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			eventType:                 "pullrequest:created",
			sourceIP:                  "1.2.3.3",
			additionalAllowedsourceIP: "1.1.3.0/16",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "10.12.33.22/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:         "check xff hijack",
			wantErr:      true,
			payloadEvent: bbcloudtest.MakePREvent("account", "sender", "sha", ""),
			eventType:    "pullrequest:created",
			sourceIP:     "1.2.3.1,127.0.0.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "1.2.3.10/16"}]}`,
					"code": "200",
				},
			},
		},
		{
			name:              "retest comment with a pipelinerun",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", "/retest dummy"),
			eventType:         "pullrequest:comment_created",
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			targetPipelinerun: "dummy",
		},
		{
			name:              "ok-to-test comment",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", "/ok-to-test"),
			eventType:         "pullrequest:comment_created",
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
		},
		{
			name:              "test comment",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", "/test"),
			eventType:         "pullrequest:comment_created",
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
		},
		{
			name:              "cancel comment with a pipelinerun",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", "/cancel dummy"),
			eventType:         "pullrequest:comment_created",
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			cancelPipelinerun: "dummy",
		},
		{
			name:              "cancel all comment",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha", "/cancel"),
			eventType:         "pullrequest:comment_created",
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &Provider{}

			req := &http.Request{Header: map[string][]string{}}
			req.Header.Set("X-Event-Key", tt.eventType)
			req.Header.Set("X-Forwarded-For", tt.sourceIP)

			run := &params.Run{
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							BitbucketCloudCheckSourceIP:      false,
							BitbucketCloudAdditionalSourceIP: "",
						},
					},
				},
			}

			if tt.sourceIP != "" {
				run.Info.Pac.BitbucketCloudCheckSourceIP = true
				run.Info.Pac.BitbucketCloudAdditionalSourceIP = tt.additionalAllowedsourceIP

				httpTestClient := httptesthelper.MakeHTTPTestClient(tt.allowedConfig)
				run.Clients.HTTP = *httpTestClient
			}

			payload, err := json.Marshal(tt.payloadEvent)
			assert.NilError(t, err)
			got, err := v.ParsePayload(ctx, run, req, string(payload))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, got != nil)
			assert.Equal(t, tt.expectedAccountID, got.AccountID)
			assert.Equal(t, tt.expectedSender, got.Sender)
			assert.Equal(t, tt.expectedSHA, got.SHA, "%s != %s", tt.expectedSHA, got.SHA)
			if tt.targetPipelinerun != "" {
				assert.Equal(t, tt.targetPipelinerun, got.TargetTestPipelineRun, tt.targetPipelinerun, got.TargetTestPipelineRun)
			}
			if tt.cancelPipelinerun != "" {
				assert.Equal(t, tt.cancelPipelinerun, got.TargetCancelPipelineRun, tt.cancelPipelinerun, got.TargetCancelPipelineRun)
			}
		})
	}
}
