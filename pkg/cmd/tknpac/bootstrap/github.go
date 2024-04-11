package bootstrap

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-github/scrape"
	"github.com/google/go-github/v61/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
)

// generateManifest generate manifest from the given options.
func generateManifest(opts *bootstrapOpts) ([]byte, error) {
	sc := scrape.AppManifest{
		Name:           github.String(opts.GithubApplicationName),
		URL:            github.String(opts.GithubApplicationURL),
		HookAttributes: map[string]string{"url": opts.RouteName},
		RedirectURL:    github.String(fmt.Sprintf("http://localhost:%d", opts.webserverPort)),
		Description:    github.String("Pipeline as Code Application"),
		Public:         github.Bool(true),
		DefaultEvents: []string{
			"check_run",
			"check_suite",
			"issue_comment",
			"commit_comment",
			triggertype.PullRequest.String(),
			"push",
		},
		DefaultPermissions: &github.InstallationPermissions{
			Checks:           github.String("write"),
			Contents:         github.String("write"),
			Issues:           github.String("write"),
			Members:          github.String("read"),
			Metadata:         github.String("read"),
			OrganizationPlan: github.String("read"),
			PullRequests:     github.String("write"),
		},
	}
	return json.Marshal(sc)
}

// getGHClient get github client.
func getGHClient(opts *bootstrapOpts) (*github.Client, error) {
	if opts.GithubAPIURL == defaultPublicGithub {
		return github.NewClient(nil), nil
	}

	gprovider, err := github.NewClient(nil).WithEnterpriseURLs(opts.GithubAPIURL, "")
	if err != nil {
		return nil, err
	}
	return gprovider, nil
}
