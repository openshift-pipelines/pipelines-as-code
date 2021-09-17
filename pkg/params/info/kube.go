package info

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type KubeOpts struct {
	ConfigPath string
	Context    string
	Namespace  string
}

func (k *KubeOpts) AddFlags(cmd *cobra.Command) {
	envkconfig := os.Getenv("KUBECONFIG")
	if envkconfig == "" {
		envkconfig = "$HOME/.kube/config"
	}
	cmd.PersistentFlags().StringVarP(
		&k.ConfigPath,
		"kubeconfig", "k", envkconfig,
		fmt.Sprintf("Path to the kubeconfig file to use for CLI requests (default: %s)", envkconfig))

	cmd.PersistentFlags().StringVarP(
		&k.ConfigPath,
		"namespace", "n", "",
		"If present, the namespace scope for this CLI request")
}
