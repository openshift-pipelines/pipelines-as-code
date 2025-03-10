package gitlab

import (
	"net/http"
	"strings"
	"testing"

	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gotest.tools/v3/assert"
)

const largeComment = "/Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s"

func TestProvider_Detect(t *testing.T) {
	sample := thelp.TEvent{
		Username:          "foo",
		DefaultBranch:     "main",
		URL:               "https://foo.com",
		SHA:               "sha",
		SHAurl:            "https://url",
		SHAtitle:          "commit it",
		Headbranch:        "branch",
		Basebranch:        "main",
		UserID:            10,
		MRID:              1,
		TargetProjectID:   100,
		SourceProjectID:   200,
		PathWithNameSpace: "hello/this/is/me/ze/project",
	}
	tests := []struct {
		name          string
		wantErrString string
		isGL          bool
		processReq    bool
		event         string
		eventType     gitlab.EventType
		wantReason    string
	}{
		{
			name:       "bad/not a gitlab Event",
			eventType:  "",
			isGL:       false,
			processReq: false,
		},
		{
			name:          "bad/invalid gitlab Event",
			eventType:     "invalid",
			wantErrString: "unexpected event type: invalid",
			isGL:          false,
			processReq:    false,
		},
		{
			name:       "good/mergeRequest open Event",
			event:      sample.MREventAsJSON("open", ""),
			eventType:  gitlab.EventTypeMergeRequest,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "bad/mergeRequest closed Event",
			event:      sample.MREventAsJSON("closed", ""),
			eventType:  gitlab.EventTypeMergeRequest,
			isGL:       true,
			processReq: false,
		},
		{
			name:       "good/mergeRequest update Event with commit",
			event:      sample.MREventAsJSON("update", `"oldrev": "123"`),
			eventType:  gitlab.EventTypeMergeRequest,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/note event",
			event:      sample.NoteEventAsJSON("abc"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "bad/note Event with ok-to-test comment",
			event:      sample.NoteEventAsJSON("/ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/issue comment Event with ok-to-test and some string",
			event:      sample.NoteEventAsJSON("abc /ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/issue comment Event with retest",
			event:      sample.NoteEventAsJSON("/retest"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/issue comment Event with cancel",
			event:      sample.NoteEventAsJSON("/cancel"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/issue comment Event with cancel a pr",
			event:      sample.NoteEventAsJSON("/cancel dummy"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/push event",
			event:      sample.PushEventAsJSON(true),
			eventType:  gitlab.EventTypePush,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/tag event",
			event:      sample.PushEventAsJSON(true),
			eventType:  gitlab.EventTypeTagPush,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "bad/commit comment unsupported action",
			event:      sample.CommitNoteEventAsJSON("/test", "update", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
			wantReason: "gitlab: commit_comment: unsupported action \"update\" with comment \"/test\"",
		},
		{
			name:       "bad/commit comment unsupported gitops command",
			event:      sample.CommitNoteEventAsJSON("/merge", "create", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
			wantReason: "gitlab: commit_comment: unsupported GitOps comment \"/merge\"",
		},
		{
			name:       "bad/commit comment unsupported large comment",
			event:      sample.CommitNoteEventAsJSON(largeComment, "create", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
			wantReason: "gitlab: commit_comment: unsupported GitOps comment \"/Lorem Ipsum is simply dummy text of the printing ...\"",
		},
		{
			name:       "good/commit comment /test command",
			event:      sample.CommitNoteEventAsJSON("/test", "create", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/commit comment /retest command",
			event:      sample.CommitNoteEventAsJSON("/retest", "create", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "good/commit comment /cancel command",
			event:      sample.CommitNoteEventAsJSON("/cancel", "create", "null"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gprovider := Provider{}
			logger, _ := logger.GetLogger()

			header := http.Header{}
			header.Set("X-Gitlab-Event", string(tt.eventType))
			req := &http.Request{Header: header}
			isGL, processReq, _, reason, err := gprovider.Detect(req, tt.event, logger)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)
			if tt.wantReason != "" {
				assert.Assert(t, strings.Contains(reason, tt.wantReason))
				return
			}
			assert.Equal(t, tt.isGL, isGL)
			assert.Equal(t, tt.processReq, processReq)
		})
	}
}
