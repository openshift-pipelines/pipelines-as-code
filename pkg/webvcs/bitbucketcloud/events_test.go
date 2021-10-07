package bitbucketcloud

import (
	"encoding/json"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	bbcloudtest "github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/bitbucketcloud/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestParsePayload1(t *testing.T) {
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
	}{
		{
			name:              "parse push request",
			payloadEvent:      bbcloudtest.MakePushEvent("PushAccountID", "Barbie", "slighlyhashed"),
			expectedSender:    "Barbie",
			expectedAccountID: "PushAccountID",
			expectedSHA:       "slighlyhashed",
			eventType:         "push",
		},
		{
			name:              "parse pull request",
			payloadEvent:      bbcloudtest.MakePREvent("TheAccountID", "Sender", "SHABidou"),
			expectedAccountID: "TheAccountID",
			expectedSender:    "Sender",
			expectedSHA:       "SHABidou",
			eventType:         "pull_request",
		},
		{
			name:              "check source ip allowed",
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha"),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pull_request",
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
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha"),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pull_request",
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
			payloadEvent:      bbcloudtest.MakePREvent("account", "sender", "sha"),
			expectedAccountID: "account",
			expectedSender:    "sender",
			expectedSHA:       "sha",
			eventType:         "pull_request",
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
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha"),
			expectedAccountID:         "account",
			expectedSender:            "sender",
			expectedSHA:               "sha",
			eventType:                 "pull_request",
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
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha"),
			expectedAccountID:         "account",
			expectedSender:            "sender",
			expectedSHA:               "sha",
			eventType:                 "pull_request",
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
			payloadEvent:              bbcloudtest.MakePREvent("account", "sender", "sha"),
			eventType:                 "pull_request",
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
			payloadEvent: bbcloudtest.MakePREvent("account", "sender", "sha"),
			eventType:    "pull_request",
			sourceIP:     "1.2.3.1,127.0.0.1",
			allowedConfig: map[string]map[string]string{
				bitbucketCloudIPrangesList: {
					"body": `{"items": [{"cidr": "1.2.3.10/16"}]}`,
					"code": "200",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &VCS{}

			event := &info.Event{
				EventType: tt.eventType,
			}
			run := &params.Run{
				Info: info.Info{
					Event: event,
				},
			}

			envDict := map[string]string{
				"PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP": "",
				"PAC_SOURCE_IP":                       "",
			}
			if tt.sourceIP != "" {
				envDict = map[string]string{
					"PAC_BITBUCKET_CLOUD_CHECK_SOURCE_IP": "True",
					"PAC_SOURCE_IP":                       tt.sourceIP,
				}
				if tt.additionalAllowedsourceIP != "" {
					envDict["PAC_BITBUCKET_CLOUD_ADDITIONAL_SOURCE_IP"] = tt.additionalAllowedsourceIP
				}

				httpTestClient := httptesthelper.MakeHTTPTestClient(t, tt.allowedConfig)
				run.Clients.HTTP = *httpTestClient
			}
			defer env.PatchAll(t, envDict)()

			payload, err := json.Marshal(tt.payloadEvent)
			assert.NilError(t, err)
			got, err := v.ParsePayload(ctx, run, string(payload))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, got != nil)
			assert.Equal(t, tt.expectedAccountID, got.AccountID)
			assert.Equal(t, tt.expectedSender, got.Sender)
			assert.Equal(t, tt.expectedSHA, got.SHA, "%s != %s", tt.expectedSHA, got.SHA)
		})
	}
}
