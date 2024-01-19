package opscomments

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	testAllRegex      = regexp.MustCompile(`(?m)^/test\s*$`)
	retestAllRegex    = regexp.MustCompile(`(?m)^/retest\s*$`)
	testSingleRegex   = regexp.MustCompile(`(?m)^/test[ \t]+\S+`)
	retestSingleRegex = regexp.MustCompile(`(?m)^/retest[ \t]+\S+`)
	oktotestRegex     = regexp.MustCompile(`(?m)^/ok-to-test\s*$`)
	cancelAllRegex    = regexp.MustCompile(`(?m)^(/cancel)\s*$`)
	cancelSingleRegex = regexp.MustCompile(`(?m)^(/cancel)[ \t]+\S+`)
)

type EventCommentType string

func (e EventCommentType) String() string {
	return string(e)
}

var (
	NoCommentEventType           = EventCommentType("")
	TestCommentEventType         = EventCommentType("test-comment")
	RetestSingleCommentEventType = EventCommentType("retest-comment")
	RetestAllCommentEventType    = EventCommentType("retest-all-comment")
	OnCommentEventType           = EventCommentType("on-comment")
	CancelCommentSingleEventType = EventCommentType("cancel-comment")
	CancelCommentAllEventType    = EventCommentType("cancel-all-comment")
	OkToTestCommentEventType     = EventCommentType("ok-to-test-comment")
)

const (
	testComment   = "/test"
	retestComment = "/retest"
	cancelComment = "/cancel"
)

func CommentEventType(comment string) EventCommentType {
	switch {
	case retestAllRegex.MatchString(comment):
		return RetestAllCommentEventType
	case retestSingleRegex.MatchString(comment):
		return RetestSingleCommentEventType
	case testAllRegex.MatchString(comment) || testSingleRegex.MatchString(comment):
		return TestCommentEventType
	case oktotestRegex.MatchString(comment):
		return OkToTestCommentEventType
	case cancelAllRegex.MatchString(comment):
		return CancelCommentAllEventType
	case cancelSingleRegex.MatchString(comment):
		return CancelCommentSingleEventType
	default:
		return NoCommentEventType
	}
}

func SetEventTypeTestPipelineRun(comment string) (string, string) {
	var eventType, targetTestPipelineRun string
	testRetestCommentType := CommentEventType(comment)
	if testRetestCommentType == NoCommentEventType {
		return eventType, targetTestPipelineRun
	}
	if testRetestCommentType == RetestSingleCommentEventType || testRetestCommentType == TestCommentEventType {
		targetTestPipelineRun = GetPipelineRunFromTestComment(comment)
	}
	return testRetestCommentType.String(), targetTestPipelineRun
}

func IsOkToTestComment(comment string) bool {
	return oktotestRegex.MatchString(comment)
}

func IsCancelComment(comment string) bool {
	return cancelAllRegex.MatchString(comment) || cancelSingleRegex.MatchString(comment)
}

func IsAnyOpsComment(eventType string) bool {
	return eventType == TestCommentEventType.String() ||
		eventType == RetestAllCommentEventType.String() ||
		eventType == RetestSingleCommentEventType.String() ||
		eventType == CancelCommentSingleEventType.String() ||
		eventType == CancelCommentAllEventType.String() ||
		eventType == OnCommentEventType.String() ||
		eventType == OkToTestCommentEventType.String()
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
	splitTest := strings.Split(comment, typeOfComment)
	// now get the first line
	getFirstLine := strings.Split(splitTest[1], "\n")
	// trim spaces
	return strings.TrimSpace(getFirstLine[0])
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
