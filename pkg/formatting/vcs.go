package formatting

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// sanitizeBranch remove refs/heads from string, only removing the first prefix
// in case we have branch that are actually called refs-heads 🙃
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
	c := cases.Title(language.AmericanEnglish)
	return strings.ReplaceAll(c.String(strings.ReplaceAll(s, "_", " ")), " ", "")
}
