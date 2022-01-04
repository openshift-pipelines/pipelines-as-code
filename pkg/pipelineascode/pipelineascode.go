package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	maxPipelineRunStatusRun = 5
	startingPipelineRunText = `Starting Pipelinerun <b>%s</b> in namespace
  <b>%s</b><br><br>You can follow the execution on the [OpenShift console](%s) pipelinerun viewer or via
  the command line with :
	<br><code>tkn pr logs -f -n %s %s</code>`
)

func Run(ctx context.Context, cs *params.Run, providerintf provider.Interface, k8int kubeinteraction.Interface) error {
	var err error

	// Match the Event URL to a Repository URL,
	repo, err := matcher.MatchEventURLRepo(ctx, cs, "")
	if err != nil {
		return err
	}

	if repo == nil || repo.Spec.URL == "" {
		msg := fmt.Sprintf("could not find a namespace match for %s", cs.Info.Event.URL)
		cs.Clients.Log.Warn(msg)

		if cs.Info.Pac.ProviderToken == "" {
			cs.Clients.Log.Warn("cannot set status since no token has been set")
			return nil
		}

		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/sad-cat-gifs",
		}
		if err := providerintf.CreateStatus(ctx, cs.Info.Event, cs.Info.Pac, status); err != nil {
			return fmt.Errorf("failed to run create status on repo not found: %w", err)
		}
		return nil
	}

	if repo.Spec.GitProvider != nil && repo.Spec.GitProvider.URL != "" {
		err := secretFromRepository(ctx, cs, k8int, providerintf.GetConfig(), repo)
		if err != nil {
			return err
		}
	} else {
		cs.Clients.Log.Infof("Using git provider %s", cs.Info.Pac.WebhookType)
	}

	// Set the client, we should error out if there is a problem with
	// token or secret or we won't be able to do much.
	err = providerintf.SetClient(ctx, cs.Info.Pac)
	if err != nil {
		return err
	}

	// Get the SHA commit info, we want to get the URL and commit title
	err = providerintf.GetCommitInfo(ctx, cs.Info.Event)
	if err != nil {
		return err
	}

	// Check if the submitter is allowed to run this.
	allowed, err := providerintf.IsAllowed(ctx, cs.Info.Event)
	if err != nil {
		return err
	}

	if !allowed {
		msg := fmt.Sprintf("User %s is not allowed to run CI on this repo.", cs.Info.Event.Sender)
		cs.Clients.Log.Info(msg)
		if cs.Info.Event.AccountID != "" {
			msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to run CI on this repo.", cs.Info.Event.Sender,
				cs.Info.Event.AccountID)
		}
		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/police-cat-gifs",
		}
		if err := providerintf.CreateStatus(ctx, cs.Info.Event, cs.Info.Pac, status); err != nil {
			return fmt.Errorf("failed to run create status, user is not allowed to run: %w", err)
		}
		return nil
	}

	pipelineRuns, err := getAllPipelineRuns(ctx, cs, providerintf)
	if err != nil {
		return err
	}
	if pipelineRuns == nil {
		msg := fmt.Sprintf("could not find templates in %s/ directory for this repository", tektonDir)
		cs.Clients.Log.Info(msg)
		return nil
	}

	// Match the pipelinerun with annotation
	pipelineRun, annotationRepo, config, err := matcher.MatchPipelinerunByAnnotation(ctx, pipelineRuns, cs)
	if err != nil {
		// Don't fail when you don't have a match between pipeline and annotations
		cs.Clients.Log.Warn(err.Error())
		return nil
	}

	if annotationRepo.Spec.URL != "" {
		repo = annotationRepo
	}

	// Automatically create a secret with the token to be reused by git-clone task
	if cs.Info.Pac.SecretAutoCreation {
		err = k8int.CreateBasicAuthSecret(ctx, cs.Info.Event, cs.Info.Pac, repo.GetNamespace())
		if err != nil {
			return fmt.Errorf("creating basic auth secret has failed: %w ", err)
		}
	}

	// Add labels and annotations to pipelinerun
	kubeinteraction.AddLabelsAndAnnotations(cs.Info.Event, pipelineRun, repo)

	// Create the actual pipeline
	pr, err := cs.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).Create(ctx, pipelineRun, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating pipelinerun %s in %s has failed: %w ", pipelineRun.GetGenerateName(), repo.GetNamespace(), err)
	}

	// Create status with the log url
	cs.Clients.Log.Infof("pipelinerun %s has been created in namespace %s for SHA: %s Target Branch: %s",
		pr.GetName(), repo.GetNamespace(), cs.Info.Event.SHA, cs.Info.Event.BaseBranch)
	consoleURL := cs.Clients.ConsoleUI.DetailURL(repo.GetNamespace(), pr.GetName())
	// Create status with the log url
	msg := fmt.Sprintf(startingPipelineRunText, pr.GetName(), repo.GetNamespace(), consoleURL, repo.GetNamespace(), pr.GetName())
	status := provider.StatusOpts{
		Status:                  "in_progress",
		Conclusion:              "pending",
		Text:                    msg,
		DetailsURL:              consoleURL,
		PipelineRunName:         pr.GetName(),
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")],
	}
	if err := providerintf.CreateStatus(ctx, cs.Info.Event, cs.Info.Pac, status); err != nil {
		return fmt.Errorf("cannot create a in_progress status on the provider platform: %w", err)
	}

	cs.Clients.Log.Infof("Waiting for PipelineRun %s/%s to Succeed in a maximum time of %s minutes",
		pr.Namespace, pr.Name, formatting.HumanDuration(cs.Info.Pac.DefaultPipelineRunTimeout))
	if err := k8int.WaitForPipelineRunSucceed(ctx, cs.Clients.Tekton.TektonV1beta1(), pr, cs.Info.Pac.DefaultPipelineRunTimeout); err != nil {
		cs.Clients.Log.Warnf("pipelinerun %s in namespace %s has a failed status",
			pipelineRun.GetGenerateName(), repo.GetNamespace())
	}

	// Do cleanups
	if keepMaxPipeline, ok := config["max-keep-runs"]; ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}

		err = k8int.CleanupPipelines(ctx, repo, pr, max)
		if err != nil {
			return err
		}
	}

	// remove the generated secret after completion of pipelinerun
	if cs.Info.Pac.SecretAutoCreation {
		err = k8int.DeleteBasicAuthSecret(ctx, cs.Info.Event, repo.GetNamespace())
		if err != nil {
			return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
		}
	}

	// Post the final status to GitHub check status with a nice breakdown and
	// tekton cli describe output.
	newPr, err := postFinalStatus(ctx, cs, providerintf, pr)
	if err != nil {
		return err
	}

	return updateRepoRunStatus(ctx, cs, newPr, repo)
}

func getAllPipelineRuns(ctx context.Context, cs *params.Run, providerintf provider.Interface) ([]*tektonv1beta1.PipelineRun, error) {
	// Get everything in tekton directory
	allTemplates, err := providerintf.GetTektonDir(ctx, cs.Info.Event, tektonDir)
	if allTemplates == "" || err != nil {
		// nolint: nilerr
		return nil, nil
	}

	// Replace those {{var}} placeholders user has in her template to the cs.Info variable
	allTemplates = templates.Process(cs.Info.Event, allTemplates)

	// Merge everything (i.e: tasks/pipeline etc..) as a single pipelinerun
	return resolve.Resolve(ctx, cs, providerintf, allTemplates, &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  cs.Info.Pac.RemoteTasks,
	})
}
