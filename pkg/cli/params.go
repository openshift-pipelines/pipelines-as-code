package cli

import (
	"github.com/pkg/errors"
	tektonversioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	pacversioned "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PacParams struct {
	clients        *Clients
	kubeConfigPath string
	kubeContext    string
	namespace      string
	githubToken    string
}

var _ Params = (*PacParams)(nil)

func (p *PacParams) SetKubeConfigPath(path string) {
	p.kubeConfigPath = path
}

func (p *PacParams) SetGitHubToken(token string) {
	p.githubToken = token
}

// Set kube client based on config
func (p *PacParams) kubeClient(config *rest.Config) (k8s.Interface, error) {
	k8scs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create k8s client from config")
	}

	return k8scs, nil
}

func (p *PacParams) dynamicClient(config *rest.Config) (dynamic.Interface, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create dynamic client from config")
	}
	return dynamicClient, err
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

func (p *PacParams) tektonClient(config *rest.Config) (tektonversioned.Interface, error) {
	cs, err := tektonversioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (p *PacParams) pacClient(config *rest.Config) (pacversioned.Interface, error) {
	cs, err := pacversioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (p *PacParams) githubClient() webvcs.GithubVCS {
	return webvcs.NewGithubVCS(p.githubToken)
}

// KubeClient returns only the kube client, not the tekton client
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

	prod, _ := zap.NewProduction()
	logger := prod.Sugar()
	defer func() {
		_ = logger.Sync() // flushes buffer, if any
	}()
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
		return nil, err
	}

	ghClient := p.githubClient()

	dynamic, err := p.dynamicClient(config)
	if err != nil {
		return nil, err
	}

	p.clients = &Clients{
		Tekton:         tekton,
		Kube:           kube,
		PipelineAsCode: pacc,
		Log:            logger,
		GithubClient:   ghClient,
		Dynamic:        dynamic,
	}

	return p.clients, nil
}
