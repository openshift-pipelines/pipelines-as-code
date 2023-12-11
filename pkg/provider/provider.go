package provider

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	testRetestAllRegex    = regexp.MustCompile(`(?m)^(/retest|/test)\s*$`)
	testRetestSingleRegex = regexp.MustCompile(`(?m)^(/test|/retest)[ \t]+\S+`)
	oktotestRegex         = regexp.MustCompile(`(?m)^/ok-to-test\s*$`)
	cancelAllRegex        = regexp.MustCompile(`(?m)^(/cancel)\s*$`)
	cancelSingleRegex     = regexp.MustCompile(`(?m)^(/cancel)[ \t]+\S+`)
)

const (
	testComment   = "/test"
	retestComment = "/retest"
	cancelComment = "/cancel"
)

const (
	GitHubApp = "GitHubApp"
)

func Valid(value string, validValues []string) bool {
	for _, v := range validValues {
		if v == value {
			return true
		}
	}
	return false
}

func IsTestRetestComment(comment string) bool {
	return testRetestSingleRegex.MatchString(comment) || testRetestAllRegex.MatchString(comment)
}

func IsOkToTestComment(comment string) bool {
	return oktotestRegex.MatchString(comment)
}

func IsCancelComment(comment string) bool {
	return cancelAllRegex.MatchString(comment) || cancelSingleRegex.MatchString(comment)
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

// CompareHostOfURLS compares the host of two parsed URLs and returns true if
// they are.
func CompareHostOfURLS(uri1, uri2 string) bool {
	u1, err := url.Parse(uri1)
	if err != nil || u1.Host == "" {
		return false
	}
	u2, err := url.Parse(uri2)
	if err != nil || u2.Host == "" {
		return false
	}
	return u1.Host == u2.Host
}
