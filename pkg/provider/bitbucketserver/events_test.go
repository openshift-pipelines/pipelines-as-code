package bitbucketserver

import (
	"encoding/json"
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
	pr1 := bbv1test.MakePREvent(ev1)

	tests := []struct {
		name          string
		payloadEvent  interface{}
		expEvent      *info.Event
		eventType     string
		wantErrSubstr string
		rawStr        string
	}{
		{
			name:          "bad/invalid event type",
			eventType:     "nono",
			payloadEvent:  bbv1.PullRequest{},
			wantErrSubstr: "event nono is not supported",
		},
		{
			name:          "bad/bad json",
			eventType:     "pull_request",
			payloadEvent:  bbv1.PullRequest{},
			rawStr:        "rageAgainst",
			wantErrSubstr: "invalid character",
		},
		{
			name:      "bad/url",
			eventType: "pull_request",
			payloadEvent: bbv1test.MakePREvent(
				&info.Event{
					AccountID:    "12345",
					Sender:       "sender",
					Organization: "PROJ",
					Repository:   "repo",
					// nolint: stylecheck
					URL: "💢",
					SHA: "abcd",
				},
			),
			wantErrSubstr: "invalid control character",
		},
		{
			name:         "good/pull_request",
			eventType:    "pull_request",
			payloadEvent: pr1,
			expEvent:     ev1,
		},
		// TODO: push test
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			v := &Provider{}
			event := &info.Event{
				EventType: tt.eventType,
			}
			run := &params.Run{
				Info: info.Info{},
			}
			_b, err := json.Marshal(tt.payloadEvent)
			assert.NilError(t, err)
			payload := string(_b)
			if tt.rawStr != "" {
				payload = tt.rawStr
			}

			got, err := v.ParsePayload(ctx, run, event, payload)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)

			assert.Equal(t, got.AccountID, tt.expEvent.AccountID)

			// test that we got slashed
			assert.Equal(t, got.URL+"/browse", tt.expEvent.URL)

			assert.Equal(t, got.CloneURL, tt.expEvent.CloneURL)
		})
	}
}
