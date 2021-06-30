package flags

import (
	"fmt"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/spf13/cobra"
)

const (
	kubeConfig = "kubeconfig"
	tokenFlag  = "token"
	apiURLFlag = "api-url"
)

// PacOptions holds struct of Pipelines as code Options
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
}

func AddWebCVSOptions(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(tokenFlag, "", os.Getenv("PAC_WEBVCS_TOKEN"),
		"Web VCS (ie: GitHub) Token")

	cmd.PersistentFlags().StringP(apiURLFlag, "", os.Getenv("PAC_WEBVCS_URL"),
		"Web VCS (ie: GitHub Enteprise) API URL")
}

func GetWebCVSOptions(p cli.Params, cmd *cobra.Command) error {
	githubToken, err := cmd.Flags().GetString(tokenFlag)
	if err != nil {
		return err
	}
	p.SetGitHubToken(githubToken)

	githubAPIURL, err := cmd.Flags().GetString(apiURLFlag)
	if err != nil {
		return err
	}
	p.SetGitHubAPIURL(githubAPIURL)
	return nil
}
