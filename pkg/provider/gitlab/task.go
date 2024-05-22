package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

type gitLabInfo struct {
	Host        string
	GroupOrUser string
	Repository  string
	Revision    string
	FilePath    string
}

// extractGitLabInfo generated with chatGPT https://chatgpt.com/share/e3c06a7e-3f16-4891-85c7-832b3e7f25c5
func extractGitLabInfo(gitlabURL string) (*gitLabInfo, error) {
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		return nil, err
	}

	// Regular expression to match the specific GitLab URL pattern
	re := regexp.MustCompile(`^/([^/]+(?:/[^/]+)*)/([^/]+)/-/blob/([^/]+)(/.*)?|^/([^/]+(?:/[^/]+)*)/([^/]+)/-/raw/([^/]+)(/.*)?`)
	matches := re.FindStringSubmatch(parsedURL.Path)

	if len(matches) == 0 {
		return nil, fmt.Errorf("URL does not match the expected GitLab pattern")
	}

	groupOrUser := ""
	repoName := ""
	revision := ""
	filePath := ""

	if matches[1] != "" { // For /blob/ URLs
		groupOrUser = matches[1]
		repoName = matches[2]
		revision = matches[3]
		if len(matches) >= 5 && matches[4] != "" {
			filePath = matches[4][1:] // Remove initial slash
		}
	} else if matches[5] != "" { // For /raw/ URLs
		groupOrUser = matches[5]
		repoName = matches[6]
		revision = matches[7]
		if len(matches) >= 9 && matches[8] != "" {
			filePath = matches[8][1:] // Remove initial slash
		}
	}

	return &gitLabInfo{
		Host:        parsedURL.Host,
		GroupOrUser: groupOrUser,
		Repository:  repoName,
		Revision:    revision,
		FilePath:    filePath,
	}, nil
}

// GetTaskURI if we are getting a URL from the same URL where the provider is,
// it means we can try to get the file with the provider token.
func (v *Provider) GetTaskURI(ctx context.Context, event *info.Event, uri string) (bool, string, error) {
	if ret := provider.CompareHostOfURLS(uri, event.URL); !ret {
		return false, "", nil
	}
	extracted, err := extractGitLabInfo(uri)
	if err != nil {
		return false, "", err
	}

	nEvent := info.NewEvent()
	nEvent.Organization = extracted.GroupOrUser
	nEvent.Repository = extracted.Repository
	nEvent.BaseBranch = extracted.Revision
	ret, err := v.GetFileInsideRepo(ctx, nEvent, extracted.FilePath, extracted.Revision)
	if err != nil {
		return false, "", err
	}
	return true, ret, nil
}
