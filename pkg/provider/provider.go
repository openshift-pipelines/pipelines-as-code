package provider

import (
	"regexp"
)

var (
	retestRegex   = regexp.MustCompile(`(?m)^/retest\s*$`)
	oktotestRegex = regexp.MustCompile(`(?m)^/ok-to-test\s*$`)
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
