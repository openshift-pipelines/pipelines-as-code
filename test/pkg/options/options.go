package options

type E2E struct {
	Repo, Organization string
	DirectWebhook      bool
	ProjectID          int
}

var (
	MainBranch       = "main"
	PullRequestEvent = "pull_request"
)
