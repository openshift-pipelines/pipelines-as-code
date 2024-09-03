package options

type E2E struct {
	Repo, Organization string
	DirectWebhook      bool
	ProjectID          int
	ControllerURL      string
	Concurrency        int
	UserName           string
	Password           string
}

var (
	MainBranch     = "main"
	RemoteTaskURL  = "https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/main/pkg/pipelineascode/testdata/pull_request/.tekton/task.yaml"
	RemoteTaskName = "task-from-tektondir"
)

const (
	InvalidYamlErrorPattern = `invalid value: ([\d\w]+) should be <= pipeline duration: ([\w\.\/\-]+)(?:, ([\w\.\/\-]+))?` +
		`|invalid value: ([\d\w]+) \+ ([\d\w]+) should be <= pipeline duration: ([\w\.\/\-]+), ([\w\.\/\-]+)` +
		`|invalid value: ([\d\w]+) should be <= pipeline duration: ([\w\.\/\-]+)`
)
