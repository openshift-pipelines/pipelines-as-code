package resolve

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	"sigs.k8s.io/yaml"
)

// nolint: gosec
const basicAuthSecretString = `secretName: "{{ git_auth_secret }}"`

func detectWebhookSecret(filenames []string) bool {
	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			return false
		}
		defer file.Close()
		// check if we have the string secretName: "{{ git_auth_secret }}" and
		// return true if it does
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), basicAuthSecretString) {
				if err := scanner.Err(); err != nil {
					return false
				}
				return true
			}
		}
	}
	return false
}

func makeGitAuthSecret(filenames []string, token string, params map[string]string) (string, string, error) {
	var ret, basicAuthsecretName string
	if !detectWebhookSecret(filenames) {
		return "", "", nil
	}
	if token == "" {
		token = os.Getenv("PAC_PROVIDER_TOKEN")
	}

	if token == "" {
		provideSecret := false
		msg := "We have detected a git_auth_secret in your Pipelinerun. Would you like to provide a token for the git_clone task?"
		if err := prompt.SurveyAskOne(&survey.Confirm{Message: msg, Default: true}, &provideSecret); err != nil {
			return "", "", fmt.Errorf("canceled")
		}
		if provideSecret {
			msg := `Enter a token to be used for the git_auth_secret`
			if err := prompt.SurveyAskOne(&survey.Password{Message: msg}, &token); err != nil {
				return "", "", fmt.Errorf("canceled")
			}
		}
	}

	if token != "" {
		runevent := &info.Event{
			URL: params["repo_url"],
			Provider: &info.Provider{
				Token: token,
			},
			SHA: params["revision"],
		}
		basicAuthsecretName = secrets.GenerateBasicAuthSecretName()
		basicAuthSecret, err := secrets.MakeBasicAuthSecret(runevent, basicAuthsecretName)
		if err != nil {
			return "", "", err
		}
		out, err := yaml.Marshal(basicAuthSecret)
		if err != nil {
			return "", "", err
		}
		ret += fmt.Sprintf("---\n%s\n", out)
	}

	return ret, basicAuthsecretName, nil
}
