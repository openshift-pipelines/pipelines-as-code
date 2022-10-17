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
	RemoteTaskURL    = "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/main/pkg/pipelineascode/testdata/pull_request/.tekton/task.yaml"
	RemoteTaskName   = "task-from-tektondir"
)
