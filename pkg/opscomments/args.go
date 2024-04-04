package opscomments

import (
	"regexp"
	"strings"
)

// ParseKeyValueArgs will parse things like key=value key="value" key="value1 value2"
// key="value1 \"value2\"" key=value1=value2.
func ParseKeyValueArgs(input string) map[string]string {
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	keyValueRegex := regexp.MustCompile(`(\w+)=(?:"([^"\\]*(?:\\.[^"\\]*)*)"|([^"'\s]+))`)
	matches := keyValueRegex.FindAllStringSubmatch(input, -1)
	keyValuePairs := make(map[string]string)

	for _, match := range matches {
		key := match[1]
		var value string
		if match[2] != "" {
			value = strings.ReplaceAll(match[2], `\"`, `"`)
		} else {
			value = match[3]
		}
		keyValuePairs[key] = value
	}

	return keyValuePairs
}
