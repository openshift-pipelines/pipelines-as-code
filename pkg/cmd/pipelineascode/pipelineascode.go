package pipelineascode

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	k8pac "github.com/openshift-pipelines/pipelines-as-code/pkg/kubernetes"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/tektoncli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type pacOptions struct {
	githubPayload string
}

var TektonDir = ".tekton"

func Command(p cli.Params) *cobra.Command {
	opts := &pacOptions{}
	var cmd = &cobra.Command{
		Use:   "run",
		Short: "Run pipelines as code",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.InitParams(p, cmd); err != nil {
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
			return runWrap(p, opts)
		},
	}

	flags.AddPacOptions(cmd)

	cmd.Flags().StringVarP(&opts.githubPayload, "github-payload", "", "", "Github Payload from webhook")
	return cmd
}

func runWrap(p cli.Params, opts *pacOptions) error {
	var runInfo = &webvcs.RunInfo{}
	cs, err := p.Clients()
	if err != nil {
		return err
	}

	err = run(p, cs, opts, runInfo)
	if err != nil {
		_, _ = cs.GithubClient.CreateStatus(runInfo, "completed", "failure",
			fmt.Sprintf("There was an issue validating the commit: %q", err),
			"https://tenor.com/search/sad-cat-gifs")
	}
	return err
}

func run(p cli.Params, cs *cli.Clients, opts *pacOptions, runinfo *webvcs.RunInfo) error {
	ctx := context.Background()
	runinfo, err := cs.GithubClient.ParsePayload(opts.githubPayload)
	if err != nil {
		return err
	}

	checkRun, err := cs.GithubClient.CreateCheckRun("in_progress", runinfo)
	if err != nil {
		return err
	}
	runinfo.CheckRunID = checkRun.ID

	op := pacpkg.PipelineAsCode{Client: cs.PipelineAsCode}
	repo, err := op.FilterBy(runinfo.URL, runinfo.Branch, "pull_request")
	if err != nil {
		return err
	}

	if repo.Spec.Namespace == "" {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "skipped",
			"Could not find a configuration for this repository", "https://tenor.com/search/sad-cat-gifs")
		cs.Log.Infof("Could not find a namespace match for %s/%s on %s", runinfo.Owner, runinfo.Repository, runinfo.Branch)
		return nil
	}

	objects, err := cs.GithubClient.GetTektonDir(TektonDir, runinfo)
	if err != nil {
		_, _ = cs.GithubClient.CreateStatus(runinfo, "completed", "skipped",
			"😿 Could not find a <b>.tekton/</b> directory for this repository", "https://tenor.com/search/sad-cat-gifs")
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

	allTemplates, err := cs.GithubClient.GetTektonDirTemplate(objects, runinfo)
	if err != nil {
		return err
	}

	allTemplates = pacpkg.ReplacePlaceHoldersVariables(allTemplates, map[string]string{
		"revision": runinfo.SHA,
		"repo_url": runinfo.URL,
	})

	prun, err := resolve.Resolve(allTemplates, true)
	if err != nil {
		return err
	}

	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, prun[0], v1.CreateOptions{})
	if err != nil {
		return err
	}

	log, err := tektoncli.FollowLogs(pr.Name, repo.Spec.Namespace, cs)
	if err != nil {
		return err
	}

	describe, err := tektoncli.PipelineRunDescribe(pr.Name, repo.Spec.Namespace)
	if err != nil {
		return err
	}
	pr, err = cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Get(ctx, pr.Name, v1.GetOptions{})

	_, err = cs.GithubClient.CreateStatus(runinfo, "completed", op.PipelineRunStatus(pr),
		"<h2>Describe output:</h2><pre>"+describe+"</pre><h2>Log output:</h2><hr><pre>"+log+"</pre>", "")

	return err
}
