package bitbucketserver

import (
	"encoding/json"
	"net/http"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	bbv1test "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/test"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestParsePayload(t *testing.T) {
	ev1 := &info.Event{
		AccountID:    "12345",
		Sender:       "sender",
		Organization: "PROJ",
		Repository:   "repo",
		URL:          "http://forge/PROJ/repo/browse",
		SHA:          "abcd",
		CloneURL:     "http://clone/PROJ/repo",
	}

	tests := []struct {
		name                    string
		payloadEvent            interface{}
		expEvent                *info.Event
		eventType               string
		wantErrSubstr           string
		rawStr                  string
		targetPipelinerun       string
		canceltargetPipelinerun string
	}{
		{
			name:          "bad/invalid event type",
			eventType:     "pr:nono",
			payloadEvent:  bbv1.PullRequest{},
			wantErrSubstr: "event \"pr:nono\" is not supported",
		},
		{
			name:          "bad/bad json",
			eventType:     "pr:opened",
			payloadEvent:  bbv1.PullRequest{},
			rawStr:        "rageAgainst",
			wantErrSubstr: "invalid character",
		},
		{
			name:      "bad/url",
			eventType: "pr:opened",
			payloadEvent: bbv1test.MakePREvent(
				&info.Event{
					AccountID:    "12345",
					Sender:       "sender",
					Organization: "PROJ",
					Repository:   "repo",
					//nolint: stylecheck
					URL: "💢",
					SHA: "abcd",
				}, ""),
			wantErrSubstr: "invalid control character",
		},
		{
			name:         "good/pull_request",
			eventType:    "pr:opened",
			payloadEvent: bbv1test.MakePREvent(ev1, ""),
			expEvent:     ev1,
		},
		{
			name:         "good/push",
			eventType:    "repo:refs_changed",
			payloadEvent: bbv1test.MakePushEvent(ev1),
			expEvent:     ev1,
		},
		{
			name:         "good/comment ok-to-test",
			eventType:    "pr:comment:added",
			payloadEvent: bbv1test.MakePREvent(ev1, "/ok-to-test"),
			expEvent:     ev1,
		},
		{
			name:         "good/comment test",
			eventType:    "pr:comment:added",
			payloadEvent: bbv1test.MakePREvent(ev1, "/test"),
			expEvent:     ev1,
		},
		{
			name:              "good/comment retest a pr",
			eventType:         "pr:comment:added",
			payloadEvent:      bbv1test.MakePREvent(ev1, "/retest dummy"),
			expEvent:          ev1,
			targetPipelinerun: "dummy",
		},
		{
			name:                    "good/comment cancel a pr",
			eventType:               "pr:comment:added",
			payloadEvent:            bbv1test.MakePREvent(ev1, "/cancel dummy"),
			expEvent:                ev1,
			canceltargetPipelinerun: "dummy",
		},
		{
			name:         "good/comment cancel all",
			eventType:    "pr:comment:added",
			payloadEvent: bbv1test.MakePREvent(ev1, "/cancel"),
			expEvent:     ev1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &Provider{}

			req := &http.Request{Header: map[string][]string{}}
			req.Header.Set("X-Event-Key", tt.eventType)

			run := &params.Run{
				Info: info.Info{},
			}
			_b, err := json.Marshal(tt.payloadEvent)
			assert.NilError(t, err)
			payload := string(_b)
			if tt.rawStr != "" {
				payload = tt.rawStr
			}

			got, err := v.ParsePayload(ctx, run, req, payload)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)

			assert.Equal(t, got.AccountID, tt.expEvent.AccountID)

			// test that we got slashed
			assert.Equal(t, got.URL+"/browse", tt.expEvent.URL)

			assert.Equal(t, got.CloneURL, tt.expEvent.CloneURL)

			if tt.targetPipelinerun != "" {
				assert.Equal(t, got.TargetTestPipelineRun, tt.targetPipelinerun)
			}
			if tt.canceltargetPipelinerun != "" {
				assert.Equal(t, got.TargetCancelPipelineRun, tt.canceltargetPipelinerun)
			}
		})
	}
}
