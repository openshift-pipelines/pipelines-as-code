package cli

import (
	"net/http"

	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/typed/pipelinesascode/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	k8s "k8s.io/client-go/kubernetes"
)

type Clients struct {
	PipelineAsCode *pacclient.PipelinesascodeV1alpha1Client
	Tekton         versioned.Interface
	Kube           k8s.Interface
	HTTPClient     http.Client
}

type Params interface {
	Clients() (*Clients, error)
	KubeClient() (k8s.Interface, error)
}
