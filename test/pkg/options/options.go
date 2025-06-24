package options

import "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"

type E2E struct {
	Repo, Organization string
	BaseBranch         string
	DirectWebhook      bool
	ProjectID          int
	ControllerURL      string
	Concurrency        int
	UserName           string
	Password           string
	Settings           v1alpha1.Settings
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
