package cli

import (
	"net/http"

	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/typed/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	k8s "k8s.io/client-go/kubernetes"
)

type Clients struct {
	PipelineAsCode *pacclient.PipelinesascodeV1alpha1Client
	Tekton         versioned.Interface
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
