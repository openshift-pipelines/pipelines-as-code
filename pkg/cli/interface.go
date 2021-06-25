package cli

import (
	"context"
	"net/http"
	"time"

	pacversioned "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonversioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonv1beta1client "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
)

type Clients struct {
	PipelineAsCode pacversioned.Interface
	Tekton         tektonversioned.Interface
	Kube           k8s.Interface
	HTTPClient     http.Client
	Log            *zap.SugaredLogger
	GithubClient   webvcs.GithubVCS
	Dynamic        dynamic.Interface
}

type Params interface {
	// SetKubeConfigPath uses the kubeconfig path to instantiate tekton
	// returned by Clientset function
	SetKubeConfigPath(string)

	// SetGitHubToken Set github token TODO: rename to a generic vcs
	SetGitHubToken(string)

	// SetGitAPIURL Set github token TODO: rename to a generic vcs
	SetGitHubAPIURL(string)

	// GetNamespace Get namesace
	GetNamespace() string

	Clients() (*Clients, error)
	KubeClient() (k8s.Interface, error)
}

type KubeInteractionIntf interface {
	GetConsoleUI(context.Context, string, string) (string, error)
	GetNamespace(context.Context, string) error
	// TODO: we don't need tektonv1beta1client stuff here
	WaitForPipelineRunSucceed(context.Context, tektonv1beta1client.TektonV1beta1Interface, *v1beta1.PipelineRun, time.Duration) error
	CleanupPipelines(context.Context, string, *webvcs.RunInfo, int) error
}
