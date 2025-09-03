package provider

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"gopkg.in/yaml.v2"
)

const ValidationErrorTemplate = `> [!CAUTION]
> There are some errors in your PipelineRun template.

| PipelineRun | Error |
|------|-------|`

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

type CommentType int

const (
	StartingPipelineType CommentType = iota
	PipelineRunStatusType
	QueueingPipelineType
)

func GetHTMLTemplate(commentType CommentType) string {
	switch commentType {
	case StartingPipelineType:
		return formatting.StartingPipelineRunHTML
	case PipelineRunStatusType:
		return formatting.PipelineRunStatusHTML
	case QueueingPipelineType:
		return formatting.QueuingPipelineRunHTML
	}
	return ""
}

func GetMarkdownTemplate(commentType CommentType) string {
	switch commentType {
	case StartingPipelineType:
		return formatting.StartingPipelineRunMarkdown
	case PipelineRunStatusType:
		return formatting.PipelineRunStatusMarkDown
	case QueueingPipelineType:
		return formatting.QueuingPipelineRunMarkdown
	}
	return ""
}

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

func GetPipelineRunAndBranchOrTagNameFromTestComment(comment string) (string, string, string, error) {
	if strings.Contains(comment, testComment) {
		return getPipelineRunAndBranchOrTagNameFromComment(testComment, comment)
	}
	return getPipelineRunAndBranchOrTagNameFromComment(retestComment, comment)
}

func GetPipelineRunAndBranchOrTagNameFromCancelComment(comment string) (string, string, string, error) {
	return getPipelineRunAndBranchOrTagNameFromComment(cancelComment, comment)
}

// getPipelineRunAndBranchOrTagNameFromComment function will take GitOps comment and split the comment
// by /test, /retest or /cancel to return branch name and pipelinerun name.
func getPipelineRunAndBranchOrTagNameFromComment(typeOfComment, comment string) (string, string, string, error) {
	var prName, branchName, tagName string
	// avoid parsing error due to branch name contains /test, /retest or /cancel,
	// here only split the first keyword and not split the later keywords.
	splitText := strings.SplitN(comment, typeOfComment, 2)

	// after the split get the second part of the typeOfComment (/test, /retest or /cancel)
	// as second part can be branch name or pipelinerun name and branch name
	// ex: /test branch:nightly, /test prname branch:nightly, /test prname branch:nightly key=value
	if splitText[1] != "" && strings.Contains(splitText[1], ":") {
		branchData := strings.Split(splitText[1], ":")

		// make sure no other word is supported other than branch word
		if !strings.Contains(splitText[1], "branch:") && !strings.Contains(splitText[1], "tag:") {
			return prName, branchName, tagName, fmt.Errorf("the GitOps comment `%s` does not contain a branch or tag word", comment)
		}

		if strings.Contains(splitText[1], "tag") {
			tagName = getBranchOrTagNameFromComment(splitText[1], "tag")
		} else {
			branchName = getBranchOrTagNameFromComment(splitText[1], "branch")
		}

		// if data after the split contains prname then fetch that
		prData := strings.Split(strings.TrimSpace(branchData[0]), " ")
		if len(prData) > 1 {
			prName = strings.TrimSpace(prData[0])
		}
	} else {
		// get the second part of the typeOfComment (/test, /retest or /cancel)
		// as second part contains pipelinerun name
		// ex: /test prname
		getFirstLine := strings.Split(splitText[1], "\n")
		// trim spaces
		// adapt for the comment contains the key=value pair
		prName = strings.Split(strings.TrimSpace(getFirstLine[0]), " ")[0]
	}
	return prName, branchName, tagName, nil
}

// getBranchOrTagNameFromComment extracts the tag or branch name that follows the "tag:" or "branch:" marker in
// the provided comment. It allows optional whitespace after the colon and
// returns the first contiguous non-whitespace token (e.g., "v1.2.3"). If no
// such token is found, it returns an empty string.
func getBranchOrTagNameFromComment(comment, prefix string) string {
	re := regexp.MustCompile(fmt.Sprintf(`%s:\s*(\S+)`, prefix))
	matches := re.FindStringSubmatch(comment)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
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

func ValidateYaml(content []byte, filename string) error {
	var validYaml any
	if err := yaml.Unmarshal(content, &validYaml); err != nil {
		return fmt.Errorf("error unmarshalling yaml file %s: %w", filename, err)
	}
	return nil
}

// GetCheckName returns the name of the check to be created based on the status
// and the pacopts.
// If the pacopts.ApplicationName is set, it will be used as the check name.
// Otherwise, the OriginalPipelineRunName will be used.
// If the OriginalPipelineRunName is not set, an empty string will be returned.
// The check name will be in the format "ApplicationName / OriginalPipelineRunName".
func GetCheckName(status StatusOpts, pacopts *info.PacOpts) string {
	if pacopts.ApplicationName != "" {
		if status.OriginalPipelineRunName == "" {
			return pacopts.ApplicationName
		}
		return fmt.Sprintf("%s / %s", pacopts.ApplicationName, status.OriginalPipelineRunName)
	}
	return status.OriginalPipelineRunName
}

func IsZeroSHA(sha string) bool {
	return sha == "0000000000000000000000000000000000000000"
}
