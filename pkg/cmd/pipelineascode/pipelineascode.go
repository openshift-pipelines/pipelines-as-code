package pipelineascode

import (
	"context"
	"errors"
	"strings"

	"github.com/google/go-github/v34/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	k8pac "github.com/openshift-pipelines/pipelines-as-code/pkg/kubernetes"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/tektoncli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type pacOptions struct {
	githubToken   string
	githubPayload string
}

// InitParams initialises cli.Params based on flags defined in command
func InitParams(p cli.Params, cmd *cobra.Command) error {
	// ensure that the config is valid by creating a client
	if _, err := p.Clients(); err != nil {
		return err
	}
	return nil
}

func Command(p cli.Params) *cobra.Command {
	opts := &pacOptions{}
	var cmd = &cobra.Command{
		Use:   "run",
		Short: "Run pipelines as code",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := InitParams(p, cmd); err != nil {
				// this check allows tkn version to be run without
				// a kubeconfig so users can verify the tkn version
				noConfigErr := strings.Contains(err.Error(), "no configuration has been provided")
				if noConfigErr {
					return nil
				}
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.githubPayload == "" {
				return errors.New("github-payload needs to be set")
			}
			if opts.githubToken == "" {
				return errors.New("github-token needs to be set")
			}
			return run(p, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.githubToken, "github-token", "", "", "Github Token used for operations")
	cmd.Flags().StringVarP(&opts.githubPayload, "github-payload", "", "", "Github Payload from webhook")
	return cmd
}

func run(p cli.Params, opts *pacOptions) error {
	ctx := context.Background()
	gvcs := webvcs.NewGithubVCS(opts.githubToken)
	cs, err := p.Clients()
	if err != nil {
		return err
	}
	runinfo, err := gvcs.ParsePayload(opts.githubPayload)
	if err != nil {
		return err
	}

	checkRun, err := gvcs.CreateCheckRun("in_progress", runinfo)
	if err != nil {
		return err
	}

	op := pacpkg.PipelineAsCode{Client: cs.PipelineAsCode}
	repo, err := op.FilterBy(runinfo.URL, runinfo.Branch, "pull_request")
	if err != nil {
		return err
	}

	if repo.Spec.Namespace == "" {
		cs.Log.Infof("Could not find a namespace match for %s/%s on %s", runinfo.Owner, runinfo.Repository, runinfo.Branch)
		return nil
	}

	objects, err := gvcs.GetTektonDir(".tekton", runinfo)
	if err != nil {
		return err
	}

	cs.Log.Infow("Loading payload",
		"url", runinfo.URL,
		"branch", runinfo.Branch,
		"sha", runinfo.SHA,
		"event_type", "pull_request")

	kcs, err := p.KubeClient()
	if err != nil {
		return err
	}

	err = k8pac.CreateNamespace(kcs, cs, repo.Spec.Namespace)
	if err != nil {
		return err
	}

	allTemplates, err := gvcs.GetTektonDirTemplate(cs, objects, runinfo)
	if err != nil {
		return err
	}

	prun, err := resolve.Resolve(allTemplates, true)
	if err != nil {
		return err
	}

	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, prun[0], v1.CreateOptions{})
	if err != nil {
		return err
	}

	err = tektoncli.FollowLogs(pr.Name, repo.Spec.Namespace, cs)
	if err != nil {
		return err
	}

	title := "CI Run Report"
	summary := "✅ CI has succeeded"
	text := "TODO"
	checkRunOutput := github.CheckRunOutput{
		Title:   &title,
		Summary: &summary,
		Text:    &text,
	}
	gvcs.CreateStatus("completed", *checkRun.ID, "success", "", &checkRunOutput, runinfo)

	return nil
}
