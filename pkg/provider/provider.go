package provider

import "regexp"

const (
	retestRegex   = "(^|\\\\r\\\\n)/retest([ ]*$|$|\\\\r\\\\n)"
	oktotestRegex = "(^|\\\\r\\\\n)/ok-to-test([ ]*$|$|\\\\r\\\\n)"
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
	matches, _ := regexp.MatchString(retestRegex, comment)
	return matches
}

func IsOkToTestComment(comment string) bool {
	matches, _ := regexp.MatchString(oktotestRegex, comment)
	return matches
}
