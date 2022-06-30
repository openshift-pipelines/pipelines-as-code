package options

type E2E struct {
	Repo, Organization string
	DirectWebhook      bool
	ProjectID          int
	ControllerURL      string
	Concurrency        int
}

var (
	MainBranch       = "main"
	PullRequestEvent = "pull_request"
	PushEvent        = "push"
)
