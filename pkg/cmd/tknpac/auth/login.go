package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/cli/v2/pkg/cmd/auth/shared"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/spf13/cobra"
)

var (
	provider  string
	authToken string
	hostname  string
	authMode  string
)

func loginCommand(_ *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "login user with provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			var username string
			var err error
			cs := ioStreams.ColorScheme()

			if provider != "github" && provider != "gitlab" && provider != "bitbucket" {
				return fmt.Errorf("provide is invalid must be amongst these three [github, gitlab, bitbucket]")
			}

			if provider != "github" {
				return fmt.Errorf("feature is in under development, at the moment only github is supported")
			}

			hosts := []string{"Github.com", "Github Enterprise Server"}
			authModes := []string{"Login with web browser", "Paste an authentication token"}

			// if user hasn't specified `--hostname` flag
			if hostname == "" {
				err = askForHostname(hosts)
				if err != nil {
					return err
				}
			}

			// if user hasn't specified token, it is needed to ask user for auth methods
			if authToken == "" {
				err = askForAuthMode(authModes)
				if err != nil {
					return err
				}
			} else {
				// if user specifies `--token`, no need to ask for auth methods
				authMode = authModes[1]
			}

			if hostname == strings.ToLower(hosts[0]) {
				hostname = defaultGithubHostname
			} else {
				hostname, err = askForEnterpriseHostName()
				if err != nil {
					return err
				}
			}

			if authMode == authModes[0] {
				authToken, username, err = RunAuthFlow(hostname, ioStreams, "", []string{}, true)
				if err != nil {
					return fmt.Errorf("failed to authenticate via web browser: %w", err)
				}
				fmt.Fprintf(ioStreams.ErrOut, "%s Authentication complete for user %s.\n", cs.SuccessIcon(), cs.GreenBold(username))
			} else {
				minimumScopes := []string{"repo", "read:org"}
				fmt.Fprintf(ioStreams.ErrOut, "Tip: you can generate a Personal Access Token here https://%s/settings/tokens, The minimum required scopes are %s.\n", hostname, scopesSentence(minimumScopes))

				if authToken == "" {
					err = askForAuthToken()
					if err != nil {
						return err
					}
				}

				// checking github permission scopes for authToken
				if err = shared.HasMinimumScopes(http.DefaultClient, hostname, authToken); err != nil {
					return fmt.Errorf("error validating token: %w", err)
				}
			}

			err = SetCred(hostname, username, authToken)
			if err != nil {
				return fmt.Errorf("error saving token in keyring: %w", err)
			}
			return nil
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}

	cmd.PersistentFlags().StringVarP(&provider, "provider", "p", "github", "Git provider possible values [github, gitlab, bitbucket]")
	cmd.PersistentFlags().StringVar(&hostname, "hostname", "", "The host name of git provider to authenticate user with")
	cmd.PersistentFlags().StringVarP(&authToken, "token", "t", "", "Read token directly from standard input")
	return cmd
}

func askForAuthToken() error {
	err := survey.AskOne(&survey.Password{
		Message: "Please enter you authentication token here:",
	}, &authToken)
	if err != nil {
		return err
	}

	return nil
}

func askForHostname(hosts []string) error {
	answers := struct {
		HostName string `survey:"hostName"`
	}{}
	qs := []*survey.Question{
		{
			Name: "hostName",
			Prompt: &survey.Select{
				Message: "Which account do you want to log in to?",
				Options: hosts,
				Default: hosts[0],
			},
		},
	}

	err := survey.Ask(qs, &answers)
	if err != nil {
		return err
	}
	hostname = strings.ToLower(answers.HostName)

	return nil
}

func askForAuthMode(authenticationMethods []string) error {
	answers := struct {
		LoginMethod string `survey:"loginMethod"`
	}{}
	qs := []*survey.Question{
		{
			Name: "loginMethod",
			Prompt: &survey.Select{
				Message: "How would you like to authenticate?",
				Options: authenticationMethods,
				Default: authenticationMethods[0],
			},
		},
	}

	err := survey.Ask(qs, &answers)
	if err != nil {
		return err
	}
	authMode = answers.LoginMethod

	return nil
}

func askForEnterpriseHostName() (string, error) {
	var hostName string
	err := survey.Ask([]*survey.Question{{
		Name:      "enterpriseHostName",
		Prompt:    &survey.Input{Message: "Enter your GHE hostname:"},
		Validate:  survey.Required,
		Transform: survey.Title,
	}}, &hostName)
	if err != nil {
		return "", err
	}

	return hostName, nil
}
