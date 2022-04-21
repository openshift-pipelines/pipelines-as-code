package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
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

type PacRun struct {
	event *info.Event
	vcx   provider.Interface
	run   *params.Run
	k8int kubeinteraction.Interface
}

func NewPacs(event *info.Event, vcx provider.Interface, run *params.Run, k8int kubeinteraction.Interface) PacRun {
	return PacRun{event: event, run: run, vcx: vcx, k8int: k8int}
}

// matchRepoPR matches the repo and the PRs from the event
func (p *PacRun) matchRepoPR(ctx context.Context) ([]matcher.Match, *v1alpha1.Repository, error) {
	// Match the Event URL to a Repository URL,
	repo, err := matcher.MatchEventURLRepo(ctx, p.run, p.event, "")
	if err != nil {
		return nil, nil, err
	}

	if repo == nil {
		if p.event.Provider.Token == "" {
			p.run.Clients.Log.Warn("cannot set status since no token has been set")
			return nil, nil, nil
		}
		msg := fmt.Sprintf("cannot find a namespace match for %s", p.event.URL)
		p.run.Clients.Log.Warn(msg)

		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/sad-cat-gifs",
		}
		if err := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status); err != nil {
			return nil, nil, fmt.Errorf("failed to run create status on repo not found: %w", err)
		}
		return nil, nil, nil
	}

	// If we have a git_provider field in repository spec, then get all the
	// information from there, including the webhook secret.
	// otherwise get the secret from the current ns (i.e: pipelines-as-code/openshift-pipelines.)
	//
	// TODO: there is going to be some improvements later we may want to do if
	// they are use cases for it :
	// allow webhook providers users to have a global webhook secret to be used,
	// so instead of having to specify their in Repo each time, they use a
	// shared one from pac.
	if repo.Spec.GitProvider != nil {
		err := secretFromRepository(ctx, p.run, p.k8int, p.vcx.GetConfig(), p.event, repo)
		if err != nil {
			return nil, nil, err
		}
	} else {
		p.event.Provider.WebhookSecret, _ = getCurrentNSWebhookSecret(ctx, p.k8int)
	}
	if err := p.vcx.Validate(ctx, p.run, p.event); err != nil {
		return nil, nil, fmt.Errorf("could not validate payload, check your webhook secret?: %w", err)
	}

	// Set the client, we should error out if there is a problem with
	// token or secret or we won't be able to do much.
	err = p.vcx.SetClient(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	// Get the SHA commit info, we want to get the URL and commit title
	err = p.vcx.GetCommitInfo(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	// Check if the submitter is allowed to run this.
	allowed, err := p.vcx.IsAllowed(ctx, p.event)
	if err != nil {
		return nil, nil, err
	}

	if !allowed {
		msg := fmt.Sprintf("User %s is not allowed to run CI on this repo.", p.event.Sender)
		p.run.Clients.Log.Info(msg)
		if p.event.AccountID != "" {
			msg = fmt.Sprintf("User: %s AccountID: %s is not allowed to run CI on this repo.", p.event.Sender, p.event.AccountID)
		}
		status := provider.StatusOpts{
			Status:     "completed",
			Conclusion: "skipped",
			Text:       msg,
			DetailsURL: "https://tenor.com/search/police-cat-gifs",
		}
		if err := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status); err != nil {
			return nil, nil, fmt.Errorf("failed to run create status, user is not allowed to run: %w", err)
		}
		return nil, nil, nil
	}

	pipelineRuns, err := getAllPipelineRuns(ctx, p.run, p.vcx, p.event)
	if err != nil {
		return nil, nil, err
	}

	if pipelineRuns == nil {
		msg := fmt.Sprintf("cannot locate templates in %s/ directory for this repository in %s", tektonDir, p.event.HeadBranch)
		p.run.Clients.Log.Info(msg)
		return nil, nil, nil
	}

	// Match the PipelineRun with annotation
	matchedPRs, err := matcher.MatchPipelinerunByAnnotation(ctx, pipelineRuns, p.run, p.event)
	if err != nil {
		// Don't fail when you don't have a match between pipeline and annotations
		// TODO: better reporting
		p.run.Clients.Log.Warn(err.Error())
		return nil, nil, nil
	}

	return matchedPRs, repo, nil
}

func (p *PacRun) Run(ctx context.Context) error {
	matchedPRs, repo, err := p.matchRepoPR(ctx)
	if err != nil {
		createStatusErr := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, provider.StatusOpts{
			Status:     "completed",
			Conclusion: "failure",
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: p.run.Clients.ConsoleUI.URL(),
		})
		if createStatusErr != nil {
			p.run.Clients.Log.Errorf("Cannot create status: %s %s", err, createStatusErr)
		}
	}

	// TODO: We need to figure out secretCreation, it's buggy normally without multiplexing ie:bug #543
	// it shows more when we do multiplex so disabling it for now.
	// we probably need a new design.
	if len(matchedPRs) > 1 {
		p.run.Clients.Log.Infof("we have matched %d pipelineruns on this event", len(matchedPRs))
		p.run.Clients.Log.Infof("disabling auto secret creation in a multiprs run")
		p.run.Info.Pac.SecretAutoCreation = false
	}

	var wg sync.WaitGroup
	for _, match := range matchedPRs {
		if match.Repo == nil {
			match.Repo = repo
		}
		wg.Add(1)

		go func(match matcher.Match) {
			defer wg.Done()
			if err := p.startPR(ctx, match); err != nil {
				p.run.Clients.Log.Errorf("PipelineRun %s has failed: %s", match.PipelineRun.GetGenerateName(), err.Error())
			}
		}(match)
	}
	wg.Wait()

	return nil
}

func (p *PacRun) startPR(ctx context.Context, match matcher.Match) error {
	// Automatically create a secret with the token to be reused by git-clone task
	if p.run.Info.Pac.SecretAutoCreation {
		if err := p.k8int.CreateBasicAuthSecret(ctx, p.event, match.Repo.GetNamespace()); err != nil {
			return fmt.Errorf("creating basic auth secret has failed: %w ", err)
		}
	}

	// Add labels and annotations to pipelinerun
	kubeinteraction.AddLabelsAndAnnotations(p.event, match.PipelineRun, match.Repo)

	// Create the actual pipeline
	pr, err := p.run.Clients.Tekton.TektonV1beta1().PipelineRuns(match.Repo.GetNamespace()).Create(ctx,
		match.PipelineRun, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating pipelinerun %s in %s has failed: %w ", match.PipelineRun.GetGenerateName(),
			match.Repo.GetNamespace(), err)
	}

	// Create status with the log url
	p.run.Clients.Log.Infof("pipelinerun %s has been created in namespace %s for SHA: %s Target Branch: %s",
		pr.GetName(), match.Repo.GetNamespace(), p.event.SHA, p.event.BaseBranch)
	consoleURL := p.run.Clients.ConsoleUI.DetailURL(match.Repo.GetNamespace(), pr.GetName())
	// Create status with the log url
	msg := fmt.Sprintf(startingPipelineRunText, pr.GetName(), match.Repo.GetNamespace(), consoleURL,
		match.Repo.GetNamespace(), pr.GetName())
	status := provider.StatusOpts{
		Status:                  "in_progress",
		Conclusion:              "pending",
		Text:                    msg,
		DetailsURL:              consoleURL,
		PipelineRunName:         pr.GetName(),
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")],
	}
	if err := p.vcx.CreateStatus(ctx, p.event, p.run.Info.Pac, status); err != nil {
		return fmt.Errorf("cannot create a in_progress status on the provider platform: %w", err)
	}

	var duration time.Duration
	if p.run.Info.Pac.DefaultPipelineRunTimeout != nil {
		duration = *p.run.Info.Pac.DefaultPipelineRunTimeout
	} else {
		// Tekton Pipeline controller should always set this value.
		duration = pr.Spec.Timeout.Duration + 1*time.Minute
	}
	p.run.Clients.Log.Infof("Waiting for PipelineRun %s/%s to Succeed in a maximum time of %s minutes",
		pr.Namespace, pr.Name, formatting.HumanDuration(duration))
	if err := p.k8int.WaitForPipelineRunSucceed(ctx, p.run.Clients.Tekton.TektonV1beta1(), pr, duration); err != nil {
		// if we have a timeout from the pipeline run, we would not know it. We would need to get the PR status to know.
		// maybe something to improve in the future.
		p.run.Clients.Log.Errorf("pipelinerun %s in namespace %s has failed: %s",
			match.PipelineRun.GetGenerateName(), match.Repo.GetNamespace(), err.Error())
	}

	// Cleanup old succeeded pipelineruns
	if keepMaxPipeline, ok := match.Config["max-keep-runs"]; ok {
		max, err := strconv.Atoi(keepMaxPipeline)
		if err != nil {
			return err
		}

		err = p.k8int.CleanupPipelines(ctx, match.Repo, pr, max)
		if err != nil {
			return err
		}
	}

	// remove the generated secret after completion of pipelinerun
	if p.run.Info.Pac.SecretAutoCreation {
		err = p.k8int.DeleteBasicAuthSecret(ctx, p.event, match.Repo.GetNamespace())
		if err != nil {
			return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
		}
	}

	// Post the final status to GitHub check status with a nice breakdown and
	// tekton cli describe output.
	newPr, err := postFinalStatus(ctx, p.run, p.vcx, p.event, pr)
	if err != nil {
		return err
	}

	return p.updateRepoRunStatus(ctx, newPr, match.Repo)
}

func getAllPipelineRuns(ctx context.Context, cs *params.Run, providerintf provider.Interface, event *info.Event) ([]*tektonv1beta1.PipelineRun, error) {
	// Get everything in tekton directory
	allTemplates, err := providerintf.GetTektonDir(ctx, event, tektonDir)
	if allTemplates == "" || err != nil {
		// nolint: nilerr
		return nil, nil
	}

	// Replace those {{var}} placeholders user has in her template to the run.Info variable
	allTemplates = templates.Process(event, allTemplates)

	// Merge everything (i.e: tasks/pipeline etc..) as a single pipelinerun
	return resolve.Resolve(ctx, cs, providerintf, event, allTemplates, &resolve.Opts{
		GenerateName: true,
		RemoteTasks:  cs.Info.Pac.RemoteTasks,
	})
}
