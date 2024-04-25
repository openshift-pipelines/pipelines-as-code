package acl

import (
	"regexp"
)

const OKToTestCommentRegexp = `(^|\n)\/ok-to-test(\r\n|\r|\n|$)`

// MatchRegexp Match a regexp to a string.
func MatchRegexp(reg, comment string) bool {
	re := regexp.MustCompile(reg)
	return string(re.Find([]byte(comment))) != ""
}
