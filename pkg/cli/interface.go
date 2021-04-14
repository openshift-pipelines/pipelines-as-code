package cli

import (
	"net/http"

	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	versionedResource "github.com/tektoncd/pipeline/pkg/client/resource/clientset/versioned"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
)

type Clients struct {
	Tekton     versioned.Interface
	Kube       k8s.Interface
	Resource   versionedResource.Interface
	HTTPClient http.Client
	Dynamic    dynamic.Interface
}

type Params interface {
	Clients() (*Clients, error)
	KubeClient() (k8s.Interface, error)
}
