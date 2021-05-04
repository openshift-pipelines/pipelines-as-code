package kubeinteraction

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	cliinterface "github.com/tektoncd/cli/pkg/cli"
	clioptions "github.com/tektoncd/cli/pkg/options"
)

type Interaction struct {
	Clients              *cli.Clients
	TektonCliLogsOptions clioptions.LogOptions
}

func (k Interaction) GetConsoleUI(ns, pr string) (string, error) {
	return consoleui.GetConsoleUI(k.Clients, ns, pr)
}

func NewKubernetesInteraction(c *cli.Clients) (*Interaction, error) {
	cliparams := &cliinterface.TektonParams{}
	_, err := cliparams.Clients()
	if err != nil {
		return nil, err
	}
	cliparams.SetNoColour(true)

	clilogoptions := clioptions.LogOptions{
		Params:   cliparams,
		AllSteps: true,
		Follow:   true,
	}

	return &Interaction{
		Clients:              c,
		TektonCliLogsOptions: clilogoptions,
	}, nil
}
