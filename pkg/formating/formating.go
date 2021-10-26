package formating

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
)

var shortShaLength = 7

func GetRepoOwnerFromGHURL(ghURL string) (string, error) {
	u, err := url.Parse(ghURL)
	if err != nil {
		return "", err
	}
	sp := strings.Split(u.Path, "/")
	if len(sp) == 1 {
		return "", fmt.Errorf("not a URL with a REPO/OWNER at the end")
	}
	return fmt.Sprintf("%s/%s", sp[len(sp)-2], sp[len(sp)-1]), nil
}

// CamelCasit pull_request > PullRequest
func CamelCasit(s string) string {
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(s, "_", " ")), " ", "")
}

func ShowStatus(repository v1alpha1.Repository, cs *cli.ColorScheme) string {
	var status string
	if len(repository.Status) == 0 {
		status = "NoRun"
	} else {
		status = repository.Status[len(repository.Status)-1].Status.Conditions[0].GetReason()
	}

	return cs.ColorStatus(status)
}

// sanitizeBranch remove refs/heads from string if it's a branch or keep it if
// it's a tag.
func SanitizeBranch(s string) string {
	if strings.HasPrefix(s, "refs/heads/") {
		return strings.TrimPrefix(s, "refs/heads/")
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

func ShowLastSHA(repository v1alpha1.Repository) string {
	if len(repository.Status) == 0 {
		return "---"
	}
	return ShortSHA(*repository.Status[len(repository.Status)-1].SHA)
}

func ShowLastAge(repository v1alpha1.Repository, cw clockwork.Clock) string {
	if len(repository.Status) == 0 {
		return "---"
	}
	return pipelineascode.Age(repository.Status[len(repository.Status)-1].CompletionTime, cw)
}
