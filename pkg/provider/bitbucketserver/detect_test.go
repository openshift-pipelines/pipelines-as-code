package bitbucketserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/types"
	"gotest.tools/v3/assert"
)

func TestProvider_Detect(t *testing.T) {
	tests := []struct {
		name          string
		wantErrString string
		isBS          bool
		processReq    bool
		event         interface{}
		eventType     string
		wantReason    string
	}{
		{
			name:       "not a bitbucket server Event",
			eventType:  "",
			isBS:       false,
			processReq: false,
		},
		{
			name:       "invalid bitbucket server Event",
			eventType:  "validator",
			isBS:       false,
			processReq: false,
		},
		{
			name: "push event",
			event: types.PushRequestEvent{
				Actor: types.EventActor{
					ID: 111,
				},
				Repository: bbv1.Repository{},
				Changes: []types.PushRequestEventChange{
					{
						ToHash: "test",
						RefID:  "refID",
					},
				},
			},
			eventType:  "repo:refs_changed",
			isBS:       true,
			processReq: true,
		},
		{
			name:       "pull_request event",
			event:      types.PullRequestEvent{},
			eventType:  "pr:opened",
			isBS:       true,
			processReq: true,
		},
		{
			name:       "updated pull_request event",
			event:      types.PullRequestEvent{},
			eventType:  "pr:from_ref_updated",
			isBS:       true,
			processReq: true,
		},
		{
			name: "retest comment",
			event: types.PullRequestEvent{
				Comment: bbv1.Comment{Text: "/retest"},
			},
			eventType:  "pr:comment:added",
			isBS:       true,
			processReq: true,
		},
		{
			name: "random comment",
			event: types.PullRequestEvent{
				Comment: bbv1.Comment{Text: "random string, ignore me :)"},
			},
			eventType:  "pr:comment:added",
			isBS:       true,
			processReq: false,
		},
		{
			name: "ok-to-test comment",
			event: types.PullRequestEvent{
				Comment: bbv1.Comment{Text: "/ok-to-test"},
			},
			eventType:  "pr:comment:added",
			isBS:       true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bprovider := Provider{}
			logger := getLogger()

			jeez, err := json.Marshal(tt.event)
			if err != nil {
				assert.NilError(t, err)
			}

			header := http.Header{}
			header.Set("X-Event-Key", tt.eventType)
			req := &http.Request{Header: header}
			isBS, processReq, _, reason, err := bprovider.Detect(req, string(jeez), logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			if tt.wantReason != "" {
				assert.Assert(t, strings.Contains(reason, tt.wantReason))
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isBS, isBS)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}
