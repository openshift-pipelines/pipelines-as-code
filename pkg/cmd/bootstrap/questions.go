package bootstrap

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

const defaultPublicGithub = "https://github.com"

func askYN(opts *bootstrapOpts, deflt bool, title, question string) (bool, error) {
	var answer bool
	// nolint:forbidigo
	fmt.Printf("%s\n", title)
	err := opts.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Confirm{
				Message: question,
				Default: deflt,
			},
		},
	}, &answer)
	if err != nil {
		return false, err
	}

	return answer, nil
}

// askQuestions ask questions to the user for the name and url of the app
func askQuestions(opts *bootstrapOpts) error {
	var qs []*survey.Question

	if opts.recreateSecret {
		answer, err := askYN(opts, false,
			fmt.Sprintf("👀 A secret named %s in %s namespace has been detected.", secretName, opts.targetNamespace),
			"Do you want me to override the secret?")
		if err != nil {
			return err
		}
		if !answer {
			return fmt.Errorf("not overriding the secret")
		}
	}

	if opts.vcsType == "github-enteprise-app" {
		prompt := "Enter your Github enteprise API URL: "
		qs = append(qs, &survey.Question{
			Name: "GithubAPIURL",
			Prompt: &survey.Input{
				Message: prompt,
			},
			Validate: survey.Required,
		})
	} else {
		opts.GithubAPIURL = defaultPublicGithub
	}

	prompt := "Enter the name of your GitHub application: "
	if opts.GithubApplicationName == "" {
		qs = append(qs, &survey.Question{
			Name:     "GithubApplicationName",
			Prompt:   &survey.Input{Message: prompt},
			Validate: survey.Required,
		})
	}

	err := opts.cliOpts.Ask(qs, opts)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(opts.GithubAPIURL, "https") {
		opts.GithubAPIURL = "https://" + strings.Trim(opts.GithubAPIURL, "/")
	}

	if opts.RouteName != "" {
		answer, err := askYN(opts, true,
			fmt.Sprintf("👀 I have detected an OpenShift Route on: %s", opts.RouteName),
			"Do you want me to use it?")
		if err != nil {
			return err
		}
		if !answer {
			opts.RouteName = ""
		}
	}

	if opts.RouteName == "" {
		err = opts.cliOpts.Ask([]*survey.Question{
			{
				Prompt: &survey.Input{
					Message: "Enter your public route URL: ",
				},
				Validate: survey.Required,
			},
		}, &opts.RouteName)
		if err != nil {
			return err
		}
	}

	if opts.GithubApplicationURL == "" {
		opts.GithubApplicationURL = opts.RouteName
	}

	return nil
}
