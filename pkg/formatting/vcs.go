package formatting

import (
	"fmt"
	"net/url"
	"strings"
)

// sanitizeBranch remove refs/heads from string if it's a branch or keep it if
// it's a tag.
func SanitizeBranch(s string) string {
	if strings.HasPrefix(s, "refs/heads/") {
		return strings.TrimPrefix(s, "refs/heads/")
	}
	if strings.HasPrefix(s, "refs-heads-") {
		return strings.TrimPrefix(s, "refs-heads-")
	}
	return s
}

// ShortSHA returns a shortsha
func ShortSHA(sha string) string {
	if sha == "" {
		return ""
	}
	if shortShaLength >= len(sha)+1 {
		return sha
	}
	return sha[0:shortShaLength]
}

func GetRepoOwnerFromGHURL(ghURL string) (string, error) {
	u, err := url.Parse(ghURL)
	if err != nil {
		return "", err
	}
	sp := strings.Split(u.Path, "/")
	if len(sp) == 1 {
		return "", fmt.Errorf("not a URL with a REPO/OWNER at the end")
	}
	return fmt.Sprintf("%s/%s", strings.ToLower(sp[len(sp)-2]), strings.ToLower(sp[len(sp)-1])), nil
}

// CamelCasit pull_request > PullRequest
func CamelCasit(s string) string {
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(s, "_", " ")), " ", "")
}
