package cli

import (
	"log"

	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/typed/pipelinesascode/v1alpha1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PacParams struct {
	clients        *Clients
	kubeConfigPath string
	kubeContext    string
	namespace      string
}

var _ Params = (*PacParams)(nil)

// Set kube client based on config
func (p *PacParams) kubeClient(config *rest.Config) (k8s.Interface, error) {
	k8scs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create k8s client from config")
	}

	return k8scs, nil
}

func (p *PacParams) config() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if p.kubeConfigPath != "" {
		loadingRules.ExplicitPath = p.kubeConfigPath
	}
	configOverrides := &clientcmd.ConfigOverrides{}
	if p.kubeContext != "" {
		configOverrides.CurrentContext = p.kubeContext
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	if p.namespace == "" {
		namespace, _, err := kubeConfig.Namespace()
		if err != nil {
			return nil, errors.Wrap(err, "Couldn't get kubeConfiguration namespace")
		}
		p.namespace = namespace
	}
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Parsing kubeconfig failed")
	}
	return config, nil
}

func (p *PacParams) tektonClient(config *rest.Config) (versioned.Interface, error) {
	cs, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (p *PacParams) pacClient(config *rest.Config) (*pacclient.PipelinesascodeV1alpha1Client, error) {
	cs, err := pacclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// Only returns kube client, not tekton client
func (p *PacParams) KubeClient() (k8s.Interface, error) {

	config, err := p.config()
	if err != nil {
		return nil, err
	}

	kube, err := p.kubeClient(config)

	if err != nil {
		return nil, err
	}

	return kube, nil
}

func (p *PacParams) Clients() (*Clients, error) {
	if p.clients != nil {
		return p.clients, nil
	}

	config, err := p.config()
	if err != nil {
		return nil, err
	}

	kube, err := p.kubeClient(config)
	if err != nil {
		return nil, err
	}

	tekton, err := p.tektonClient(config)
	if err != nil {
		return nil, err
	}

	pacc, err := p.pacClient(config)
	if err != nil {
		log.Fatal(err)
	}

	p.clients = &Clients{
		Tekton: tekton,
		Kube:   kube,
		Pac:    pacc,
	}

	return p.clients, nil
}
