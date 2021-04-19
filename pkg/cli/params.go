package cli

import (
	"github.com/pkg/errors"
	"github.com/tektoncd/hub/api/pkg/cli/hub"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"

	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/typed/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
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

func (p *PacParams) hubClient(config *rest.Config) hub.Client {
	cs := hub.NewClient()
	return cs
}

func (p *PacParams) githubClient(config *rest.Config) (webvcs.GithubVCS, error) {
	return webvcs.NewGithubVCS(p.githubToken), nil
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

	hub := p.hubClient(config)

	ghClient, err := p.githubClient(config)
	if err != nil {
		return nil, err
	}

	p.clients = &Clients{
		Tekton:         tekton,
		Kube:           kube,
		PipelineAsCode: pacc,
		Log:            logger,
		GithubClient:   ghClient,
		Hub:            hub,
	}

	return p.clients, nil
}
