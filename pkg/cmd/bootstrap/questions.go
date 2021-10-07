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

	if opts.RecreateSecret {
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

	prompt := "Enter the name of your GitHub application: "
	if opts.ApplicationName == "" {
		qs = append(qs, &survey.Question{
			Name:   "ApplicationName",
			Prompt: &survey.Input{Message: prompt},
		})
	}
	if opts.ApplicationURL == "" {
		prompt = "Enter an URL for your GitHub application"
		qs = append(qs, &survey.Question{
			Name:   "ApplicationURL",
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
