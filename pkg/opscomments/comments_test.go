package opscomments

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
)

func TestLabelsBackwardCompat(t *testing.T) {
	testCases := []struct {
		name     string
		label    string
		expected string
	}{
		{
			name:     "OnCommentEventType",
			label:    OnCommentEventType.String(),
			expected: OnCommentEventType.String(),
		},
		{
			name:     "RetestAllCommentEventType",
			label:    RetestAllCommentEventType.String(),
			expected: triggertype.PullRequest.String(),
		},
		{
			name:     "OtherLabel",
			label:    "OtherLabel",
			expected: "OtherLabel",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)

			sampleRepo := testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-repo-already-installed",
				InstallNamespace: "namespace",
				URL:              "https://pac.test/already/installed",
			})
			tdata := testclient.Data{Repositories: []*v1alpha1.Repository{sampleRepo}}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			eventEmitter := events.NewEventEmitter(stdata.Kube, logger)
			result := EventTypeBackwardCompat(eventEmitter, sampleRepo, tc.label)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetNameFromFunction(t *testing.T) {
	tests := []struct {
		name        string
		comment     string
		commentType string
		expected    string
		wantErr     bool
	}{
		{
			name:        "get name from test comment",
			comment:     "/test prname",
			expected:    "prname",
			commentType: testComment,
		},
		{
			name:        "get name from test comment with args",
			comment:     "/test prname foo=bar hello=moto",
			expected:    "prname",
			commentType: testComment,
		},
		{
			name:        "get name from cancel comment",
			comment:     "/cancel prname",
			expected:    "prname",
			commentType: cancelComment,
		},
		{
			name:        "get name from retest comment",
			comment:     "/retest prname",
			expected:    "prname",
			commentType: retestComment,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNameFromComment(tt.commentType, tt.comment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAnyOpsEventType(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      bool
	}{
		{
			name:      "TestSingleCommentEventType",
			eventType: TestSingleCommentEventType.String(),
			want:      true,
		},
		{
			name:      "TestAllCommentEventType",
			eventType: TestAllCommentEventType.String(),
			want:      true,
		},
		{
			name:      "RetestAllCommentEventType",
			eventType: RetestAllCommentEventType.String(),
			want:      true,
		},
		{
			name:      "RetestSingleCommentEventType",
			eventType: RetestSingleCommentEventType.String(),
			want:      true,
		},
		{
			name:      "CancelCommentSingleEventType",
			eventType: CancelCommentSingleEventType.String(),
			want:      true,
		},
		{
			name:      "CancelCommentAllEventType",
			eventType: CancelCommentAllEventType.String(),
			want:      true,
		},
		{
			name:      "OkToTestCommentEventType",
			eventType: OkToTestCommentEventType.String(),
			want:      true,
		},
		{
			name:      "OnCommentEventType",
			eventType: OnCommentEventType.String(),
			want:      true,
		},
		{
			name:      "NoOpsCommentEventType",
			eventType: NoOpsCommentEventType.String(),
			want:      false,
		},
		{
			name:      "Random string",
			eventType: "random string",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAnyOpsEventType(tt.eventType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCommentEventTypeTest(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    EventType
	}{
		{
			name:    "retest all",
			comment: "/retest",
			want:    RetestAllCommentEventType,
		},
		{
			name:    "retest single",
			comment: "/retest prname",
			want:    RetestSingleCommentEventType,
		},
		{
			name:    "test all",
			comment: "/test",
			want:    TestAllCommentEventType,
		},
		{
			name:    "test single",
			comment: "/test prname",
			want:    TestSingleCommentEventType,
		},
		{
			name:    "ok to test",
			comment: "/ok-to-test",
			want:    OkToTestCommentEventType,
		},
		{
			name:    "no comment event type",
			comment: "random comment",
			want:    NoOpsCommentEventType,
		},
		{
			name:    "cancel all",
			comment: "/cancel",
			want:    CancelCommentAllEventType,
		},
		{
			name:    "cancel single",
			comment: "/cancel prname",
			want:    CancelCommentSingleEventType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommentEventType(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetEventTypeTestPipelineRun(t *testing.T) {
	tests := []struct {
		name         string
		comment      string
		wantType     string
		wantTestPr   string
		wantCancelPr string
		wantCancel   bool
	}{
		{
			name:     "no event type",
			comment:  "random comment",
			wantType: NoOpsCommentEventType.String(),
		},
		{
			name:       "retest single event type",
			comment:    "/retest prname",
			wantType:   RetestSingleCommentEventType.String(),
			wantTestPr: "prname",
		},
		{
			name:       "test single event type",
			comment:    "/test prname",
			wantType:   TestSingleCommentEventType.String(),
			wantTestPr: "prname",
		},
		{
			name:         "cancel single pr",
			comment:      "/cancel prname",
			wantType:     CancelCommentSingleEventType.String(),
			wantCancelPr: "prname",
			wantCancel:   true,
		},
		{
			name:       "cancel all pr",
			comment:    "/cancel",
			wantType:   CancelCommentAllEventType.String(),
			wantCancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &info.Event{}
			SetEventTypeAndTargetPR(event, tt.comment)
			assert.Equal(t, tt.wantType, event.EventType)
			assert.Equal(t, tt.wantTestPr, event.TargetTestPipelineRun)
		})
	}
}

func TestIsOkToTestComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid",
			comment: "/ok-to-test",
			want:    true,
		},
		{
			name:    "valid with some string before",
			comment: "/lgtm \n/ok-to-test",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/ok-to-test \n then report the status back",
			want:    true,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/ok-to-test \n/approve",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
		{
			name:    "invalid comment",
			comment: "/ok-to-test abc",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOkToTestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCancelComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "valid",
			comment: "/cancel",
			want:    true,
		},
		{
			name:    "valid with some string before",
			comment: "/lgtm \n/cancel",
			want:    true,
		},
		{
			name:    "valid with some string before and after",
			comment: "hi, trigger the ci \n/cancel \n then report the status back",
			want:    true,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/cancel \n/approve",
			want:    true,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    false,
		},
		{
			name:    "invalid comment",
			comment: "/ok-to-test abc",
			want:    false,
		},
		{
			name:    "cancel single pr",
			comment: "/cancel abc",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCancelComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTestRetestComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    EventType
	}{
		{
			name:    "valid retest",
			comment: "/retest",
			want:    RetestAllCommentEventType,
		},
		{
			name:    "valid test all",
			comment: "/test",
			want:    TestAllCommentEventType,
		},
		{
			name:    "valid retest with some string before",
			comment: "/lgtm \n/retest",
			want:    RetestAllCommentEventType,
		},
		{
			name:    "valid test with some string before",
			comment: "/lgtm \n/test",
			want:    TestAllCommentEventType,
		},
		{
			name:    "retest with some string before and after",
			comment: "hi, trigger the ci \n/retest \n then report the status back",
			want:    RetestAllCommentEventType,
		},
		{
			name:    "test valid with some string before and after",
			comment: "hi, trigger the ci \n/test \n then report the status back",
			want:    TestAllCommentEventType,
		},
		{
			name:    "test valid with some string before and after",
			comment: "hi, trigger the ci \n/test foobar\n then report the status back",
			want:    TestSingleCommentEventType,
		},
		{
			name:    "valid comments",
			comment: "/lgtm \n/retest \n/approve",
			want:    RetestAllCommentEventType,
		},
		{
			name:    "retest trigger single pr",
			comment: "/retest abc",
			want:    RetestSingleCommentEventType,
		},
		{
			name:    "test trigger single pr",
			comment: "/test abc",
			want:    TestSingleCommentEventType,
		},
		{
			name:    "invalid",
			comment: "test abc",
			want:    NoOpsCommentEventType,
		},
		{
			name:    "invalid",
			comment: "/ok",
			want:    NoOpsCommentEventType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommentEventType(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunFromComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "test no pipelinerun",
			comment: "/test",
			want:    "",
		},
		{
			name:    "retest no pipelinerun",
			comment: "/retest",
			want:    "",
		},
		{
			name:    "test a pipeline",
			comment: "/test abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before test command",
			comment: "abc \n /test abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after test command",
			comment: "/test abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after test command",
			comment: "before \n /test abc-01-pr \n after",
			want:    "abc-01-pr",
		},
		{
			name:    "retest a pipeline",
			comment: "/retest abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before retest command",
			comment: "abc \n /retest abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after retest command",
			comment: "/retest abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after retest command",
			comment: "before \n /retest abc-01-pr \n after",
			want:    "abc-01-pr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPipelineRunFromTestComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunFromCancelComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "cancel all",
			comment: "/cancel",
			want:    "",
		},
		{
			name:    "cancel a pipeline",
			comment: "/cancel abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string before cancel command",
			comment: "abc \n /cancel abc-01-pr",
			want:    "abc-01-pr",
		},
		{
			name:    "string after cancel command",
			comment: "/cancel abc-01-pr \n abc",
			want:    "abc-01-pr",
		},
		{
			name:    "string before and after cancel command",
			comment: "before \n /cancel abc-01-pr \n after",
			want:    "abc-01-pr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPipelineRunFromCancelComment(tt.comment)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelineRunAndBranchNameFromTestComment(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		branchName string
		prName     string
		wantError  bool
	}{
		{
			name:       "retest all on test branch",
			comment:    "/retest branch:test",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "test a pipeline on test branch",
			comment:    "/test abc-01-pr branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for test command before branch name test",
			comment:    "/test abc-01-pr abc \n branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for retest command after branch name test",
			comment:    "/retest abc-01-pr branch:test \n abc",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for test command before and after branch name test",
			comment:    "/test abc-01-pr \n before branch:test \n after",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:      "different word other than branch for retest command",
			comment:   "/retest invalidname:nightly",
			wantError: true,
		},
		{
			name:      "test all",
			comment:   "/test",
			wantError: false,
		},
		{
			name:      "test a pipeline",
			comment:   "/test abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string before retest command",
			comment:   "abc \n /retest abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string after retest command",
			comment:   "/retest abc-01-pr \n abc",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "string before and after test command",
			comment:   "before \n /test abc-01-pr \n after",
			prName:    "abc-01-pr",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prName, branchName, err := GetPipelineRunAndBranchNameFromTestComment(tt.comment)
			assert.Equal(t, tt.wantError, err != nil)
			assert.Equal(t, tt.branchName, branchName)
			assert.Equal(t, tt.prName, prName)
		})
	}
}

func TestGetPipelineRunAndBranchNameFromCancelComment(t *testing.T) {
	tests := []struct {
		name       string
		comment    string
		branchName string
		prName     string
		wantError  bool
	}{
		{
			name:      "cancel all pipeline",
			comment:   "/cancel",
			wantError: false,
		},
		{
			name:      "cancel a particular pipeline",
			comment:   "/cancel abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string before cancel command",
			comment:   "abc \n /cancel abc-01-pr",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string after cancel command",
			comment:   "/cancel abc-01-pr \n abc",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:      "add string before and after cancel command",
			comment:   "before \n /cancel abc-01-pr \n after",
			prName:    "abc-01-pr",
			wantError: false,
		},
		{
			name:       "cancel all on test branch",
			comment:    "/cancel branch:test",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "cancel a pipeline on test branch",
			comment:    "/cancel abc-01-pr branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command before branch name test",
			comment:    "/cancel abc-01-pr abc \n branch:test",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command after branch name test",
			comment:    "/cancel abc-01-pr branch:test \n abc",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:       "string for cancel command before and after branch name test",
			comment:    "/cancel abc-01-pr \n before branch:test \n after",
			prName:     "abc-01-pr",
			branchName: "test",
			wantError:  false,
		},
		{
			name:      "different word other than branch for cancel command",
			comment:   "/cancel invalidname:nightly",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prName, branchName, err := GetPipelineRunAndBranchNameFromCancelComment(tt.comment)
			assert.Equal(t, tt.wantError, err != nil)
			assert.Equal(t, tt.branchName, branchName)
			assert.Equal(t, tt.prName, prName)
		})
	}
}

func TestAnyOpsKubeLabelInSelector(t *testing.T) {
	assert.Assert(t, strings.Contains(AnyOpsKubeLabelInSelector(), RetestSingleCommentEventType.String()))
}
