package provider

import (
	"regexp"
	"strings"
)

var (
	retestRegex   = regexp.MustCompile(`(?m)^/retest\s*$`)
	oktotestRegex = regexp.MustCompile(`(?m)^/ok-to-test\s*$`)
	testRegex     = regexp.MustCompile(`(?m)^/test[ \t]+\S+`)
)

const (
	ProviderGitHubApp     = "GitHubApp"
	ProviderGitHubWebhook = "GitHubWebhook"
	ProviderGitLabWebhook = "GitLabWebhook"
)

func Valid(value string, validValues []string) bool {
	for _, v := range validValues {
		if v == value {
			return true
		}
	}
	return false
}

func IsRetestComment(comment string) bool {
	return retestRegex.MatchString(comment)
}

func IsOkToTestComment(comment string) bool {
	return oktotestRegex.MatchString(comment)
}

func IsTestComment(comment string) bool {
	return testRegex.MatchString(comment)
}

func GetPipelineRunFromComment(comment string) string {
	// get string after /test command
	splitTest := strings.Split(comment, "/test")
	// now get the first line
	getFirstLine := strings.Split(splitTest[1], "\n")
	// trim spaces
	return strings.TrimSpace(getFirstLine[0])
}
