package pipelineascode

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir               = ".tekton"
	startingPipelineRunText = `Starting Pipelinerun <b>%s</b> in namespace
  <b>%s</b><br><br>You can follow the execution on the [OpenShift console](%s) pipelinerun viewer or via
  the command line with :
	<br><code>tkn pr logs -f -n %s %s</code>`
	queuingPipelineRunText = `PipelineRun <b>%s</b> has been queued Queuing in namespace
  <b>%s</b><br><br>`
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
		createStatusErr := p.vcx.CreateStatus(ctx, p.run.Clients.Tekton, p.event, p.run.Info.Pac, provider.StatusOpts{
			Status:     "completed",
			Conclusion: "failure",
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: p.run.Clients.ConsoleUI.URL(),
		})
		if createStatusErr != nil {
			p.logger.Errorf("cannot create status: %s: %s", err.Error(), createStatusErr.Error())
		} else {
			p.logger.Infof("reported error on provider status: %s", err.Error())
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
	kubeinteraction.AddLabelsAndAnnotations(p.event, match.PipelineRun, match.Repo, p.vcx.GetConfig())

	// if concurrency is defined then start the pipelineRun in pending state and
	// state as queued
	if match.Repo.Spec.ConcurrencyLimit != nil && *match.Repo.Spec.ConcurrencyLimit != 0 {
		// pending status
		match.PipelineRun.Spec.Status = v1beta1.PipelineRunSpecStatusPending
		// pac state as queued
		match.PipelineRun.Labels[filepath.Join(apipac.GroupName, "state")] = kubeinteraction.StateQueued
	}

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
		PipelineRun:             pr,
		OriginalPipelineRunName: pr.GetLabels()[filepath.Join(apipac.GroupName, "original-prname")],
	}

	// if pipelineRun is in pending state then report status as queued
	if pr.Spec.Status == v1beta1.PipelineRunSpecStatusPending {
		status.Status = "queued"
		status.Text = fmt.Sprintf(queuingPipelineRunText, pr.GetName(), match.Repo.GetNamespace())
	}

	if err := p.vcx.CreateStatus(ctx, p.run.Clients.Tekton, p.event, p.run.Info.Pac, status); err != nil {
		return fmt.Errorf("cannot create a in_progress status on the provider platform: %w", err)
	}
	return nil
}
