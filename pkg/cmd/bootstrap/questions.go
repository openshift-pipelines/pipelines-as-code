package bootstrap

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

func askYN(opts *bootstrapOpts, title, question string) (bool, error) {
	var answer bool
	// nolint:forbidigo
	fmt.Printf("%s\n", title)
	err := opts.cliOpts.Ask([]*survey.Question{
		{
			Prompt: &survey.Confirm{
				Message: question,
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
		answer, err := askYN(opts,
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
			Name:   "GithubAPIURL",
			Prompt: &survey.Input{Message: prompt},
		})
	}

	prompt := "Enter the name of your GitHub application: "
	if opts.GithubApplicationName == "" {
		qs = append(qs, &survey.Question{
			Name:   "GithubApplicationName",
			Prompt: &survey.Input{Message: prompt},
		})
	}
	if opts.GithubApplicationURL == "" {
		prompt = "Enter an URL for your GitHub application"
		qs = append(qs, &survey.Question{
			Name:   "GithubApplicationURL",
			Prompt: &survey.Input{Message: prompt},
		})
	}

	err := opts.cliOpts.Ask(qs, opts)
	if err != nil {
		return err
	}

	if opts.RouteName != "" {
		answer, err := askYN(opts,
			fmt.Sprintf("👀 We have detected an OpenShift Route on: %s", opts.RouteName),
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
			},
		}, &opts.RouteName)
		if err != nil {
			return err
		}
	}

	return nil
}
