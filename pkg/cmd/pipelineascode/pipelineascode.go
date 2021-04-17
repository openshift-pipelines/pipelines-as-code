package pipelineascode

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	pacpkg "github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
	cliinterface "github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/log"
	clilog "github.com/tektoncd/cli/pkg/log"
	clioptions "github.com/tektoncd/cli/pkg/options"
	kcorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	cs.Log.Infof("Target Namespace is: %s", repo.Spec.Namespace)
	var all_objects string
	var all_templates string
	// I miss map/lambda :(
	for _, value := range objects {
		if all_objects != "" {
			all_objects += ", "
		}
		all_objects += value.GetName()
		if value.GetName() != "tekton.yaml" && (strings.HasSuffix(value.GetName(), ".yaml") ||
			strings.HasSuffix(value.GetName(), ".yml")) {
			data, err := gvcs.GetObject(value.GetSHA(), runinfo)
			if err != nil {
				cs.Log.Fatal(err)
			}
			if all_templates != "" && !strings.HasPrefix(string(data), "---") {
				all_templates += "---"
			}
			all_templates += "\n" + string(data)
		}
	}
	cs.Log.Infof("Templates in .tekton directory: %s", all_objects)
	kcs, err := p.KubeClient()
	if err != nil {
		return err
	}

	_, err = kcs.CoreV1().Namespaces().Get(context.Background(), repo.Spec.Namespace, v1.GetOptions{})
	if err != nil {
		cs.Log.Infof("Creating Namespace: %s", repo.Spec.Namespace)
		_, err = kcs.CoreV1().Namespaces().Create(context.Background(), &kcorev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: repo.Spec.Namespace,
			},
		}, v1.CreateOptions{})
		if err != nil {
			return (err)
		}
	}

	prun, err := resolve.Resolve(all_templates, true)
	if err != nil {
		return err
	}

	pr, err := cs.Tekton.TektonV1beta1().PipelineRuns(repo.Spec.Namespace).Create(ctx, prun[0], v1.CreateOptions{})
	if err != nil {
		return err
	}

	cliparam := cliinterface.TektonParams{}
	cliparam.SetNamespace(repo.Spec.Namespace)
	cliparam.Clients()
	cliparam.SetNoColour(true)
	cliopts := clioptions.LogOptions{
		Params:          &cliparam,
		AllSteps:        true,
		PipelineRunName: pr.Name,
		Follow:          true,
	}
	lr, err := clilog.NewReader(clilog.LogTypePipeline, &cliopts)

	logC, errC, err := lr.Read()
	if err != nil {
		return err
	}

	cs.Log.Infof("Watching PipelineRun %s", pr.Name)

	cliopts.Stream = &cliinterface.Stream{
		Out: os.Stdout,
		Err: os.Stderr,
	}

	log.NewWriter(log.LogTypePipeline).Write(cliopts.Stream, logC, errC)

	return nil
}
