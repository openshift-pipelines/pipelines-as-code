package cli

import (
	"net/http"

	pacversioned "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/tektoncli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/hub/api/pkg/cli/hub"
	tektonversioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	k8s "k8s.io/client-go/kubernetes"
)

type Clients struct {
	PipelineAsCode pacversioned.Interface
	Tekton         tektonversioned.Interface
	TektonCli      tektoncli.Interface
	Hub            hub.Client
	Kube           k8s.Interface
	HTTPClient     http.Client
	Log            *zap.SugaredLogger
	GithubClient   webvcs.GithubVCS
}

type Params interface {
	// SetKubeConfigPath uses the kubeconfig path to instantiate tekton
	// returned by Clientset function
	SetKubeConfigPath(string)

	// SetGitHubToken Set github token TODO: rename to a generic vcs
	SetGitHubToken(string)

	Clients() (*Clients, error)
	KubeClient() (k8s.Interface, error)
}
