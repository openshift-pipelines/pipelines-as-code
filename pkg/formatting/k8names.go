package formatting

import (
	"regexp"
	"strings"
)

// CleanKubernetesName takes a string and performs the following actions to make it a valid
// Kubernetes resource name:
//
// 1. Converts the string to lowercase.
// 2. Trims leading and trailing whitespace.
// 3. Replaces any characters that are not lowercase alphanumeric characters, '-', or '.' with '-'.
//
// The resulting string is a valid Kubernetes resource name.
// Reference https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names

func CleanKubernetesName(s string) string {
	regex := regexp.MustCompile(`[^a-z0-9\.-]`)
	s = strings.TrimSpace(strings.ToLower(s))
	replaced := regex.ReplaceAllString(s, "-")
	return replaced
}
