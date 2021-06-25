package flags

import (
	"fmt"
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

	return nil
}

// AddPacOptions amends command to add flags required to initialize a cli.Param
func AddPacOptions(cmd *cobra.Command) {
	envkconfig := os.Getenv("KUBECONFIG")
	if envkconfig == "" {
		envkconfig = "$HOME/.kube/config"
	}

	cmd.PersistentFlags().StringP(
		kubeConfig, "k", "",
		fmt.Sprintf("kubectl config file (default: %s)", envkconfig))

	cmd.PersistentFlags().StringP(
		token, "", os.Getenv("PAC_WEBVCS_TOKEN"),
		"Web VCS (ie: GitHub) Token")

	cmd.PersistentFlags().StringP(
		apiURL, "", os.Getenv("PAC_WEBVCS_URL"),
		"Web VCS (ie: GitHub Enteprise) API URL")
}
