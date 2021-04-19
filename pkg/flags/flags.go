package flags

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

const (
	kubeConfig  = "kubeconfig"
	githubToken = "github-token"
)

type PacOptions struct {
	KubeConfig, GithubToken string
}

// InitParams initialises cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	kcPath, err := cmd.Flags().GetString(kubeConfig)
	if err != nil {
		return err
	}
	p.SetKubeConfigPath(kcPath)

	githubToken, err := cmd.Flags().GetString(githubToken)
	if err != nil {
		return err
	}
	p.SetGitHubToken(githubToken)

	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}

	return nil
}

// AddTektonOptions amends command to add flags required to initialise a cli.Param
func AddPacOptions(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(
		kubeConfig, "k", "",
		"kubectl config file (default: $HOME/.kube/config)")
	cmd.PersistentFlags().StringP(
		githubToken, "", "",
		"GitHub Token")
}
