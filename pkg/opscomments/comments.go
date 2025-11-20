package opscomments

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/acl"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
)

var (
	testAllRegex      = regexp.MustCompile(`(?m)^/test\s*$`)
	retestAllRegex    = regexp.MustCompile(`(?m)^/retest\s*$`)
	testSingleRegex   = regexp.MustCompile(`(?m)^/test[ \t]+\S+`)
	retestSingleRegex = regexp.MustCompile(`(?m)^/retest[ \t]+\S+`)
	oktotestRegex     = regexp.MustCompile(acl.OKToTestCommentRegexp)
	cancelAllRegex    = regexp.MustCompile(`(?m)^(/cancel)\s*$`)
	cancelSingleRegex = regexp.MustCompile(`(?m)^(/cancel)[ \t]+\S+`)
)

type EventType string

func (e EventType) String() string {
	return string(e)
}

var (
	NoOpsCommentEventType        = EventType("no-ops-comment")
	TestAllCommentEventType      = EventType("test-all-comment")
	TestSingleCommentEventType   = EventType("test-comment")
	RetestSingleCommentEventType = EventType("retest-comment")
	RetestAllCommentEventType    = EventType("retest-all-comment")
	OnCommentEventType           = EventType("on-comment")
	CancelCommentSingleEventType = EventType("cancel-comment")
	CancelCommentAllEventType    = EventType("cancel-all-comment")
	OkToTestCommentEventType     = EventType("ok-to-test-comment")
)

const (
	testComment   = "/test"
	retestComment = "/retest"
	cancelComment = "/cancel"
)

func CommentEventType(comment string) EventType {
	switch {
	case retestAllRegex.MatchString(comment):
		return RetestAllCommentEventType
	case retestSingleRegex.MatchString(comment):
		return RetestSingleCommentEventType
	case testAllRegex.MatchString(comment):
		return TestAllCommentEventType
	case testSingleRegex.MatchString(comment):
		return TestSingleCommentEventType
	case oktotestRegex.MatchString(comment):
		return OkToTestCommentEventType
	case cancelAllRegex.MatchString(comment):
		return CancelCommentAllEventType
	case cancelSingleRegex.MatchString(comment):
		return CancelCommentSingleEventType
	default:
		return NoOpsCommentEventType
	}
}

// SetEventTypeAndTargetPR function will set the event type and target test pipeline run in an event.
func SetEventTypeAndTargetPR(event *info.Event, comment string) {
	commentType := CommentEventType(comment)
	if commentType == RetestSingleCommentEventType || commentType == TestSingleCommentEventType {
		event.TargetTestPipelineRun = GetPipelineRunFromTestComment(comment)
	}
	if commentType == CancelCommentAllEventType || commentType == CancelCommentSingleEventType {
		event.CancelPipelineRuns = true
	}
	if commentType == CancelCommentSingleEventType {
		event.TargetCancelPipelineRun = GetPipelineRunFromCancelComment(comment)
	}
	event.EventType = commentType.String()
	event.TriggerComment = comment
}

func IsOkToTestComment(comment string) bool {
	return oktotestRegex.MatchString(comment)
}

// GetSHAFromOkToTestComment extracts the optional SHA from an /ok-to-test comment.
func GetSHAFromOkToTestComment(comment string) string {
	matches := oktotestRegex.FindStringSubmatch(comment)
	if len(matches) > 2 {
		return strings.TrimSpace(matches[2])
	}
	return ""
}

// EventTypeBackwardCompat handle the backward compatibility we need to keep until
// we have done the deprecated notice
//
// 2024-07-01 chmouel
//
//	set anyOpsComments to pull_request see https://issues.redhat.com/browse/SRVKP-5775
//	we keep on-comment to the "on-comment" type
func EventTypeBackwardCompat(eventEmitter *events.EventEmitter, repo *v1alpha1.Repository, label string) string {
	if label == OnCommentEventType.String() {
		return label
	}
	if IsAnyOpsEventType(label) {
		eventEmitter.EmitMessage(repo, zap.WarnLevel, "DeprecatedOpsComment",
			fmt.Sprintf("the %s event type is deprecated, this will be changed to %s in the future",
				label, triggertype.PullRequest.String()))
		return triggertype.PullRequest.String()
	}
	return label
}

func IsAnyOpsEventType(eventType string) bool {
	return eventType == TestSingleCommentEventType.String() ||
		eventType == TestAllCommentEventType.String() ||
		eventType == RetestAllCommentEventType.String() ||
		eventType == RetestSingleCommentEventType.String() ||
		eventType == CancelCommentSingleEventType.String() ||
		eventType == CancelCommentAllEventType.String() ||
		eventType == OkToTestCommentEventType.String() ||
		eventType == OnCommentEventType.String()
}

// AnyOpsKubeLabelInSelector will output a Kubernetes label out of all possible
// CommentEvent Type for selection.
func AnyOpsKubeLabelInSelector() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s",
		TestSingleCommentEventType.String(),
		TestAllCommentEventType.String(),
		RetestAllCommentEventType.String(),
		RetestSingleCommentEventType.String(),
		CancelCommentSingleEventType.String(),
		CancelCommentAllEventType.String(),
		OkToTestCommentEventType.String(),
		OnCommentEventType.String())
}

func GetPipelineRunFromTestComment(comment string) string {
	if strings.Contains(comment, testComment) {
		return getNameFromComment(testComment, comment)
	}
	return getNameFromComment(retestComment, comment)
}

func GetPipelineRunFromCancelComment(comment string) string {
	return getNameFromComment(cancelComment, comment)
}

func getNameFromComment(typeOfComment, comment string) string {
	splitTest := strings.Split(strings.TrimSpace(comment), typeOfComment)
	if len(splitTest) < 2 {
		return ""
	}
	// now get the first line
	getFirstLine := strings.Split(splitTest[1], "\n")

	// and the first argument
	firstArg := strings.Split(getFirstLine[0], " ")
	if len(firstArg) < 2 {
		return ""
	}

	// trim spaces
	return strings.TrimSpace(firstArg[1])
}

func GetPipelineRunAndBranchNameFromTestComment(comment string) (string, string, error) {
	if strings.Contains(comment, testComment) {
		return getPipelineRunAndBranchNameFromComment(testComment, comment)
	}
	return getPipelineRunAndBranchNameFromComment(retestComment, comment)
}

func GetPipelineRunAndBranchNameFromCancelComment(comment string) (string, string, error) {
	return getPipelineRunAndBranchNameFromComment(cancelComment, comment)
}

// getPipelineRunAndBranchNameFromComment function will take GitOps comment and split the comment
// by /test, /retest or /cancel to return branch name and pipelinerun name.
func getPipelineRunAndBranchNameFromComment(typeOfComment, comment string) (string, string, error) {
	var prName, branchName string
	splitTest := strings.Split(comment, typeOfComment)

	// after the split get the second part of the typeOfComment (/test, /retest or /cancel)
	// as second part can be branch name or pipelinerun name and branch name
	// ex: /test branch:nightly, /test prname branch:nightly
	if splitTest[1] != "" && strings.Contains(splitTest[1], ":") {
		branchData := strings.Split(splitTest[1], ":")

		// make sure no other word is supported other than branch word
		if !strings.Contains(branchData[0], "branch") {
			return prName, branchName, fmt.Errorf("the GitOps comment%s does not contain a branch word", branchData[0])
		}
		branchName = strings.Split(strings.TrimSpace(branchData[1]), " ")[0]

		// if data after the split contains prname then fetch that
		prData := strings.Split(strings.TrimSpace(branchData[0]), " ")
		if len(prData) > 1 {
			prName = strings.TrimSpace(prData[0])
		}
	} else {
		// get the second part of the typeOfComment (/test, /retest or /cancel)
		// as second part contains pipelinerun name
		// ex: /test prname
		getFirstLine := strings.Split(splitTest[1], "\n")
		// trim spaces
		prName = strings.TrimSpace(getFirstLine[0])
	}
	return prName, branchName, nil
}
