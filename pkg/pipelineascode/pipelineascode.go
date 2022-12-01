package pipelineascode

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	tektonDir = ".tekton"
)

type PacRun struct {
	event        *info.Event
	vcx          provider.Interface
	run          *params.Run
	k8int        kubeinteraction.Interface
	logger       *zap.SugaredLogger
	eventEmitter *events.EventEmitter
}

func NewPacs(event *info.Event, vcx provider.Interface, run *params.Run, k8int kubeinteraction.Interface, logger *zap.SugaredLogger) PacRun {
	return PacRun{
		event: event, run: run, vcx: vcx, k8int: k8int, logger: logger,
		eventEmitter: events.NewEventEmitter(run.Clients.Kube, logger),
	}
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
			p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryCreateStatus", fmt.Sprintf("Cannot create status: %s: %s", err, createStatusErr))
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
				errMsg := fmt.Sprintf("PipelineRun %s has failed: %s", match.PipelineRun.GetGenerateName(), err.Error())
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", errMsg)
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
		if annotation, ok := match.PipelineRun.GetAnnotations()[keys.GitAuthSecret]; ok {
			gitAuthSecretName = annotation
		} else {
			return fmt.Errorf("cannot get annotation %s as set on PR", keys.GitAuthSecret)
		}

		authSecret, err := secrets.MakeBasicAuthSecret(p.event, gitAuthSecretName)
		if err != nil {
			return err
		}

		if err = p.k8int.CreateSecret(ctx, match.Repo.GetNamespace(), authSecret); err != nil {
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
		match.PipelineRun.Labels[keys.State] = kubeinteraction.StateQueued
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
	msg := fmt.Sprintf(params.StartingPipelineRunText, pr.GetName(), match.Repo.GetNamespace(), consoleURL,
		match.Repo.GetNamespace(), match.Repo.GetName())
	status := provider.StatusOpts{
		Status:                  "in_progress",
		Conclusion:              "pending",
		Text:                    msg,
		DetailsURL:              consoleURL,
		PipelineRunName:         pr.GetName(),
		PipelineRun:             pr,
		OriginalPipelineRunName: pr.GetLabels()[keys.OriginalPRName],
	}

	// if pipelineRun is in pending state then report status as queued
	if pr.Spec.Status == v1beta1.PipelineRunSpecStatusPending {
		status.Status = "queued"
		status.Text = fmt.Sprintf(params.QueuingPipelineRunText, pr.GetName(), match.Repo.GetNamespace())
	}

	if err := p.vcx.CreateStatus(ctx, p.run.Clients.Tekton, p.event, p.run.Info.Pac, status); err != nil {
		return fmt.Errorf("cannot create a in_progress status on the provider platform: %w", err)
	}

	// Patch pipelineRun with logURL annotation, skips for GitHub App as we patch logURL while patching checkrunID
	if _, ok := pr.Annotations[keys.InstallationID]; !ok {
		return patchPipelineRunWithLogURL(ctx, p.logger, p.run.Clients, pr)
	}
	return nil
}

func patchPipelineRunWithLogURL(ctx context.Context, logger *zap.SugaredLogger, clients clients.Clients, pr *v1beta1.PipelineRun) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mergePatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					keys.LogURL: clients.ConsoleUI.DetailURL(pr.GetNamespace(), pr.GetName()),
				},
			},
		}
		patch, err := json.Marshal(mergePatch)
		if err != nil {
			return err
		}
		patchedPR, err := clients.Tekton.TektonV1beta1().PipelineRuns(pr.GetNamespace()).Patch(ctx, pr.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logger.Infof("could not patch Pipelinerun with log URL, retrying %v/%v: %v", pr.GetNamespace(), pr.GetName(), err)
			return err
		}
		logger.Infof("patched log URL to pipelinerun: %v/%v", patchedPR.Namespace, patchedPR.Name)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to patch log url to pipelinerun %v/%v: %w", pr.Namespace, pr.Name, err)
	}
	return nil
}
