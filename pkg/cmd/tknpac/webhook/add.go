package webhook

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/webhook"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/tknpac/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var namespaceFlag = "namespace"

func webhookAdd(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	var pacNamespace string
	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{""},
		Short:   "Adds a webhook secret on the git provider settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				err      error
				repoName string
			)
			opts := cli.NewCliOptions(cmd)

			opts.Namespace, err = cmd.Flags().GetString(namespaceFlag)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if len(args) > 0 {
				repoName = args[0]
			}
			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}

			return add(ctx, opts, run, ioStreams, repoName, pacNamespace)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.PersistentFlags().StringVarP(&pacNamespace, "pac-namespace",
		"", "", "The namespace where pac is installed")

	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	_ = cmd.RegisterFlagCompletionFunc(namespaceFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespaceFlag, args)
		},
	)
	return cmd
}

func add(ctx context.Context, opts *cli.PacCliOpts, run *params.Run, ioStreams *cli.IOStreams, repoName, pacNamespace string) error {
	var (
		err          error
		repo         *v1alpha1.Repository
		providerName string
	)
	if opts.Namespace != "" {
		run.Info.Kube.Namespace = opts.Namespace
	}

	if repoName != "" {
		repo, err = run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(run.Info.Kube.Namespace).Get(ctx,
			repoName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		repo, err = prompt.SelectRepo(ctx, run, run.Info.Kube.Namespace)
		if err != nil {
			return err
		}
	}

	// Should not proceed when GithubApp is configured or GitProvider is nil
	if repo.Spec.GitProvider == nil {
		fmt.Fprintf(ioStreams.Out, "%s Webhook is not configured for the repository %s ",
			ioStreams.ColorScheme().InfoIcon(),
			repoName)
		return nil
	}

	if repo.Spec.GitProvider.Secret == nil {
		fmt.Fprintf(ioStreams.Out, "%s Can not configure webhook as git_provider secret is empty",
			ioStreams.ColorScheme().WarningIcon())
		return nil
	}

	secretName := repo.Spec.GitProvider.Secret.Name
	secretData, err := run.Clients.Kube.CoreV1().Secrets(repo.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	gitProviderSecretKey := repo.Spec.GitProvider.Secret.Key
	if gitProviderSecretKey == "" {
		gitProviderSecretKey = pipelineascode.DefaultGitProviderSecretKey
	}

	tokenData, ok := secretData.Data[gitProviderSecretKey]
	if !ok {
		fmt.Fprintf(ioStreams.Out, "Token is empty, You can use the command \"tkn pac webhook update-token\" to update the provider token in %s secret", repoName)
		return nil
	}

	msg := "Please enter the provider name to setup the webhook:"
	providerLabels := make([]string, 0, len(webhook.ProviderTypes))
	for _, label := range webhook.ProviderTypes {
		providerLabels = append(providerLabels, label)
	}
	if err := prompt.SurveyAskOne(
		&survey.Select{
			Message: msg,
			Options: providerLabels,
			Default: 0,
		}, &providerName); err != nil {
		return err
	}

	config := &webhook.Options{
		Run:                      run,
		RepositoryName:           repo.Name,
		RepositoryNamespace:      repo.Namespace,
		PACNamespace:             pacNamespace,
		RepositoryURL:            repo.Spec.URL,
		ProviderAPIURL:           repo.Spec.GitProvider.URL,
		IOStreams:                ioStreams,
		PersonalAccessToken:      string(tokenData),
		RepositoryCreateORUpdate: false,
		SecretName:               secretName,
		ProviderSecretKey:        gitProviderSecretKey,
	}

	return config.Install(ctx, providerName)
}
