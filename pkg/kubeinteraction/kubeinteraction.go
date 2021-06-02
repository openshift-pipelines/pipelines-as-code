package kubeinteraction

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
)

type Interaction struct {
	Clients *cli.Clients
}

func (k Interaction) GetConsoleUI(ctx context.Context, ns, pr string) (string, error) {
	return consoleui.GetConsoleUI(ctx, k.Clients, ns, pr)
}

func NewKubernetesInteraction(c *cli.Clients) (*Interaction, error) {
	return &Interaction{
		Clients: c,
	}, nil
}
