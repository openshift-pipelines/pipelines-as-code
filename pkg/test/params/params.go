package params

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

type FakeParams struct {
	Fakeclients *cli.Clients
}

// SetKubeConfigPath uses the kubeconfig path to instantiate tekton
// returned by Clientset function
func (p FakeParams) SetKubeConfigPath(string) {
}

// SetGitHubToken Set github token TODO: rename to a generic vcs
func (p FakeParams) SetGitHubToken(string) {
}

// SetGitHubAPIURL Set github token TODO: rename to a generic vcs
func (p FakeParams) SetGitHubAPIURL(string) {
}

func (p FakeParams) Clients() (*cli.Clients, error) {
	return p.Fakeclients, nil
}

func (p FakeParams) KubeClient() (kubernetes.Interface, error) {
	return nil, nil
}
