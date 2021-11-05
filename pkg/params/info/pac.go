package info

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type PacOpts struct {
	LogURL               string
	ApplicationName      string // the Application Name for example "Pipelines as Code"
	SecretAutoCreation   bool   // secret auto creation in target namespace
	ProviderToken        string
	ProviderURL          string
	ProviderUser         string
	ProviderInfoFromRepo bool // whether the provider info come from the repository
	WebhookType          string
	PayloadFile          string
	TektonDashboardURL   string
	HubURL               string
}

func (p *PacOpts) AddFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().StringVarP(&p.WebhookType, "git-provider-type", "",
		os.Getenv("PAC_GIT_PROVIDER_TYPE"),
		"Webhook type")

	providerToken := os.Getenv("PAC_GIT_PROVIDER_TOKEN")
	if providerToken != "" {
		if _, err := os.Stat(providerToken); !os.IsNotExist(err) {
			data, err := ioutil.ReadFile(providerToken)
			if err != nil {
				return err
			}
			providerToken = string(data)
		}
	}

	cmd.PersistentFlags().StringVarP(&p.ProviderToken, "git-provider-token", "", providerToken,
		"Git Provider Token")

	cmd.PersistentFlags().StringVarP(&p.ProviderURL, "git-provider-api-url", "",
		os.Getenv("PAC_GIT_PROVIDER_APIURL"),
		"Git Provider API URL")

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
