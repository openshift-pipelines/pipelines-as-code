package info

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type PacOpts struct {
	LogURL             string
	ApplicationName    string // the Application Name for example "Pipelines as Code"
	SecretAutoCreation bool   // secret auto creation in target namespace
	VCSToken           string
	VCSAPIURL          string
	VCSType            string
	PayloadFile        string
}

func (p *PacOpts) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&p.VCSType, "webvcs-type", "", os.Getenv("PAC_WEBVCS_TYPE"),
		"Web VCS (ie: GitHub) Token")

	cmd.PersistentFlags().StringVarP(&p.VCSToken, "webvcs-token", "", os.Getenv("PAC_WEBVCS_TOKEN"),
		"Web VCS (ie: GitHub) Token")

	cmd.PersistentFlags().StringVarP(&p.VCSAPIURL, "webvcs-api-url", "", os.Getenv("PAC_WEBVCS_URL"),
		"Web VCS (ie: GitHub Enteprise) API URL")

	cmd.PersistentFlags().StringVarP(&p.PayloadFile,
		"payload-file", "", os.Getenv("PAC_PAYLOAD_FILE"), "A file containing the webhook payload")

	applicationName := os.Getenv("PAC_APPLICATION_NAME")
	if applicationName == "" {
		applicationName = defaultApplicationName
	}
	cmd.Flags().StringVar(&p.ApplicationName,
		"application-name", applicationName,
		"The name of the application.")

	secretAutoCreation := false
	secretAutoCreationEnv := os.Getenv("PAC_SECRET_AUTO_CREATE")
	if strings.ToLower(secretAutoCreationEnv) == "true" ||
		strings.ToLower(secretAutoCreationEnv) == "yes" || secretAutoCreationEnv == "1" {
		secretAutoCreation = true
	}
	cmd.Flags().BoolVar(&p.SecretAutoCreation,
		"secret-auto-creation",
		secretAutoCreation,
		"Wether to create automatically secrets.")
}
