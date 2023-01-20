package gitlab

import (
	"net/http"
	"strings"
	"testing"

	thelp "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab/test"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
)

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
			name:       "not a gitlab Event",
			eventType:  "",
			isGL:       false,
			processReq: false,
		},
		{
			name:          "invalid gitlab Event",
			eventType:     "invalid",
			wantErrString: "unexpected event type: invalid",
			isGL:          false,
			processReq:    false,
		},
		{
			name:       "valid merge Event",
			event:      sample.MREventAsJSON(),
			eventType:  gitlab.EventTypeMergeRequest,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue note event with no valid comment",
			event:      sample.NoteEventAsJSON("abc"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
		},
		{
			name:       "issue note Event with ok-to-test comment",
			event:      sample.NoteEventAsJSON("/ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue comment Event with ok-to-test and some string",
			event:      sample.NoteEventAsJSON("abc /ok-to-test"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: false,
		},
		{
			name:       "issue comment Event with retest",
			event:      sample.NoteEventAsJSON("/retest"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue comment Event with cancel",
			event:      sample.NoteEventAsJSON("/cancel"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "issue comment Event with cancel a pr",
			event:      sample.NoteEventAsJSON("/cancel dummy"),
			eventType:  gitlab.EventTypeNote,
			isGL:       true,
			processReq: true,
		},
		{
			name:       "push event",
			event:      sample.PushEventAsJSON(true),
			eventType:  gitlab.EventTypePush,
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
