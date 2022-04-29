package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"go.uber.org/zap"
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
	event  *info.Event
	vcx    provider.Interface
	run    *params.Run
	k8int  kubeinteraction.Interface
	logger *zap.SugaredLogger
}

func NewPacs(event *info.Event, vcx provider.Interface, run *params.Run, k8int kubeinteraction.Interface, logger *zap.SugaredLogger) PacRun {
	return PacRun{event: event, run: run, vcx: vcx, k8int: k8int, logger: logger}
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
			p.logger.Errorf("Cannot create status: %s %s", err, createStatusErr)
		}
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
				p.logger.Errorf("PipelineRun %s has failed: %s", match.PipelineRun.GetGenerateName(), err.Error())
			}
		}(match)
	}
	wg.Wait()

	return nil
}

func (p *PacRun) startPR(ctx context.Context, match matcher.Match) error {
	var gitAuthSecretName string

	// Automatically create a secret with the token to be reused by git-clone task
	if p.run.Info.Pac.SecretAutoCreation {
		if annotation, ok := match.PipelineRun.GetAnnotations()[gitAuthSecretAnnotation]; ok {
			gitAuthSecretName = annotation
		} else {
			return fmt.Errorf("cannot get annotation %s as set on PR", gitAuthSecretAnnotation)
		}

		var err error
		if err = p.k8int.CreateBasicAuthSecret(ctx, p.logger, p.event, match.Repo.GetNamespace(), gitAuthSecretName); err != nil {
			return fmt.Errorf("creating basic auth secret: %s has failed: %w ", gitAuthSecretName, err)
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
	p.logger.Infof("pipelinerun %s has been created in namespace %s for SHA: %s Target Branch: %s",
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
	p.logger.Infof("Waiting for PipelineRun %s/%s to Succeed in a maximum time of %s minutes",
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

		err = p.k8int.CleanupPipelines(ctx, p.logger, match.Repo, pr, max)
		if err != nil {
			return err
		}
	}

	// remove the generated secret after completion of pipelinerun
	if p.run.Info.Pac.SecretAutoCreation {
		err = p.k8int.DeleteBasicAuthSecret(ctx, p.logger, match.Repo.GetNamespace(), gitAuthSecretName)
		if err != nil {
			return fmt.Errorf("deleting basic auth secret has failed: %w ", err)
		}
	}

	// Post the final status to GitHub check status with a nice breakdown and
	// tekton cli describe output.
	newPr, err := p.postFinalStatus(ctx, pr)
	if err != nil {
		return err
	}

	return p.updateRepoRunStatus(ctx, newPr, match.Repo)
}
