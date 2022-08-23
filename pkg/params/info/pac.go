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
	WebhookType        string
	PayloadFile        string
	TektonDashboardURL string
	HubURL             string
	RemoteTasks        bool

	// bitbucket cloud specific fields
	BitbucketCloudCheckSourceIP      bool
	BitbucketCloudAdditionalSourceIP string
}

func (p *PacOpts) AddFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().StringVarP(&p.WebhookType, "git-provider-type", "",
		os.Getenv("PAC_GIT_PROVIDER_TYPE"),
		"Webhook type")

	cmd.PersistentFlags().StringVarP(&p.PayloadFile,
		"payload-file", "", os.Getenv("PAC_PAYLOAD_FILE"), "A file containing the webhook payload")

	applicationName := os.Getenv("PAC_APPLICATION_NAME")
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

	return nil
}
