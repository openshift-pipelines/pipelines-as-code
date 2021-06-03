package flags

import (
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

const (
	kubeConfig = "kubeconfig"
	token      = "token"
	apiURL     = "api-url"
)

// PacOptions holds struct of Pipeline as code Options
type PacOptions struct {
	KubeConfig, GithubToken, GithubAPIURL string
}

// InitParams initializes cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	kcPath, err := cmd.Flags().GetString(kubeConfig)
	if err != nil {
		return err
	}
	p.SetKubeConfigPath(kcPath)

	githubToken, err := cmd.Flags().GetString(token)
	if err != nil {
		return err
	}
	p.SetGitHubToken(githubToken)

	githubAPIURL, err := cmd.Flags().GetString(apiURL)
	if err != nil {
		return err
	}
	p.SetGitHubAPIURL(githubAPIURL)

	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}

	return nil
}

// AddPacOptions amends command to add flags required to initialize a cli.Param
func AddPacOptions(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(
		kubeConfig, "k", "",
		"kubectl config file (default: $HOME/.kube/config)")

	cmd.PersistentFlags().StringP(
		token, "", os.Getenv("PAC_TOKEN"),
		"Web VCS (ie: GitHub) Token")

	cmd.PersistentFlags().StringP(
		apiURL, "", os.Getenv("PAC_URL"),
		"Web VCS (ie: GitHub Enteprise) API URL")
}
