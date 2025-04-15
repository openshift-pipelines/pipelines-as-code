package pipelineascode

import (
	"context"
	"fmt"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/action"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/customparams"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/secrets"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tektonDir         = ".tekton"
	CompletedStatus   = "completed"
	inProgressStatus  = "in_progress"
	queuedStatus      = "queued"
	failureConclusion = "failure"
	pendingConclusion = "pending"
	neutralConclusion = "neutral"
)

type PacRun struct {
	event        *info.Event
	vcx          provider.Interface
	run          *params.Run
	k8int        kubeinteraction.Interface
	logger       *zap.SugaredLogger
	eventEmitter *events.EventEmitter
	manager      *ConcurrencyManager
	pacInfo      *info.PacOpts
	globalRepo   *v1alpha1.Repository
}

func NewPacs(event *info.Event, vcx provider.Interface, run *params.Run, pacInfo *info.PacOpts, k8int kubeinteraction.Interface, logger *zap.SugaredLogger, globalRepo *v1alpha1.Repository) PacRun {
	return PacRun{
		event: event, run: run, vcx: vcx, k8int: k8int, pacInfo: pacInfo, logger: logger, globalRepo: globalRepo,
		eventEmitter: events.NewEventEmitter(run.Clients.Kube, logger),
		manager:      NewConcurrencyManager(),
	}
}

func (p *PacRun) Run(ctx context.Context) error {
	matchedPRs, repo, err := p.matchRepoPR(ctx)
	if repo != nil && p.event.TriggerTarget == triggertype.PullRequestClosed {
		if err := p.cancelAllInProgressBelongingToClosedPullRequest(ctx, repo); err != nil {
			return fmt.Errorf("error cancelling in progress pipelineRuns belonging to pull request %d: %w", p.event.PullRequestNumber, err)
		}
		return nil
	}
	if err != nil {
		createStatusErr := p.vcx.CreateStatus(ctx, p.event, provider.StatusOpts{
			Status:     CompletedStatus,
			Conclusion: failureConclusion,
			Text:       fmt.Sprintf("There was an issue validating the commit: %q", err),
			DetailsURL: p.run.Clients.ConsoleUI().URL(),
		})
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryCreateStatus", fmt.Sprintf("an error occurred: %s", err))
		if createStatusErr != nil {
			p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryCreateStatus", fmt.Sprintf("cannot create status: %s: %s", err, createStatusErr))
		}
	}
	if len(matchedPRs) == 0 {
		return nil
	}
	if repo.Spec.ConcurrencyLimit != nil && *repo.Spec.ConcurrencyLimit != 0 {
		p.manager.Enable()
	}

	// set params for the console driver, only used for the custom console ones
	cp := customparams.NewCustomParams(p.event, repo, p.run, p.k8int, p.eventEmitter, p.vcx)
	maptemplate, _, err := cp.GetParams(ctx)
	if err != nil {
		p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "ParamsError",
			fmt.Sprintf("error processing repository CR custom params: %s", err.Error()))
	}
	p.run.Clients.ConsoleUI().SetParams(maptemplate)

	var wg sync.WaitGroup
	for i, match := range matchedPRs {
		if match.Repo == nil {
			match.Repo = repo
		}

		// After matchRepo func fetched repo from k8s api repo is updated and
		// need to merge global repo again
		if p.globalRepo != nil {
			match.Repo.Spec.Merge(p.globalRepo.Spec)
		}

		wg.Add(1)

		go func(match matcher.Match, i int) {
			defer wg.Done()
			pr, err := p.startPR(ctx, match)
			if err != nil {
				errMsg := fmt.Sprintf("There was an error starting the PipelineRun %s, %s", match.PipelineRun.GetGenerateName(), err.Error())
				errMsgM := fmt.Sprintf("There was an error creating the PipelineRun: <b>%s</b>\n\n%s", match.PipelineRun.GetGenerateName(), err.Error())
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", errMsg)
				createStatusErr := p.vcx.CreateStatus(ctx, p.event, provider.StatusOpts{
					Status:                   CompletedStatus,
					Conclusion:               failureConclusion,
					Text:                     errMsgM,
					DetailsURL:               p.run.Clients.ConsoleUI().URL(),
					InstanceCountForCheckRun: i,
				})
				if createStatusErr != nil {
					p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryCreateStatus", fmt.Sprintf("Cannot create status: %s: %s", err, createStatusErr))
				}
			}
			p.manager.AddPipelineRun(pr)
			if err := p.cancelInProgressMatchingPipelineRun(ctx, pr, repo); err != nil {
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", fmt.Sprintf("error cancelling in progress pipelineRuns: %s", err))
			}
		}(match, i)
	}
	wg.Wait()

	order, prs := p.manager.GetExecutionOrder()
	if order != "" {
		for _, pr := range prs {
			wg.Add(1)

			go func(order string, pr tektonv1.PipelineRun) {
				defer wg.Done()
				if _, err := action.PatchPipelineRun(ctx, p.logger, "execution order", p.run.Clients.Tekton, &pr, getExecutionOrderPatch(order)); err != nil {
					errMsg := fmt.Sprintf("Failed to patch pipelineruns %s execution order: %s", pr.GetGenerateName(), err.Error())
					p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", errMsg)
					return
				}
			}(order, *pr)
		}
	}
	wg.Wait()
	return nil
}

func (p *PacRun) startPR(ctx context.Context, match matcher.Match) (*tektonv1.PipelineRun, error) {
	var gitAuthSecretName string

	// Automatically create a secret with the token to be reused by git-clone task
	if p.pacInfo.SecretAutoCreation {
		if annotation, ok := match.PipelineRun.GetAnnotations()[keys.GitAuthSecret]; ok {
			gitAuthSecretName = annotation
		} else {
			return nil, fmt.Errorf("cannot get annotation %s as set on PR", keys.GitAuthSecret)
		}

		authSecret, err := secrets.MakeBasicAuthSecret(p.event, gitAuthSecretName)
		if err != nil {
			return nil, fmt.Errorf("making basic auth secret: %s has failed: %w ", gitAuthSecretName, err)
		}

		if err = p.k8int.CreateSecret(ctx, match.Repo.GetNamespace(), authSecret); err != nil {
			return nil, fmt.Errorf("creating basic auth secret: %s has failed: %w ", authSecret.GetName(), err)
		}
	}

	// Add labels and annotations to pipelinerun
	err := kubeinteraction.AddLabelsAndAnnotations(p.event, match.PipelineRun, match.Repo, p.vcx.GetConfig(), p.run)
	if err != nil {
		p.logger.Errorf("Error adding labels/annotations to PipelineRun '%s' in namespace '%s': %v", match.PipelineRun.GetName(), match.Repo.GetNamespace(), err)
	}

	// if concurrency is defined then start the pipelineRun in pending state and
	// state as queued
	if match.Repo.Spec.ConcurrencyLimit != nil && *match.Repo.Spec.ConcurrencyLimit != 0 {
		// pending status
		match.PipelineRun.Spec.Status = tektonv1.PipelineRunSpecStatusPending
		// pac state as queued
		match.PipelineRun.Labels[keys.State] = kubeinteraction.StateQueued
		match.PipelineRun.Annotations[keys.State] = kubeinteraction.StateQueued
	}

	// Create the actual pipelineRun
	pr, err := p.run.Clients.Tekton.TektonV1().PipelineRuns(match.Repo.GetNamespace()).Create(ctx,
		match.PipelineRun, metav1.CreateOptions{})
	if err != nil {
		// cleanup the gitauth secret because ownerRef isn't set when the pipelineRun creation failed
		if p.pacInfo.SecretAutoCreation {
			if errDelSec := p.k8int.DeleteSecret(ctx, p.logger, match.Repo.GetNamespace(), gitAuthSecretName); errDelSec != nil {
				// don't overshadow the pipelineRun creation error, just log
				p.logger.Errorf("removing auto created secret: %s in namespace %s has failed: %w ", gitAuthSecretName, match.Repo.GetNamespace(), errDelSec)
			}
		}
		// we need to make difference between markdown error and normal error that goes to namespace/controller stream
		return nil, fmt.Errorf("creating pipelinerun %s in namespace %s has failed.\n\nTekton Controller has reported this error: ```%w``` ", match.PipelineRun.GetGenerateName(),
			match.Repo.GetNamespace(), err)
	}

	// update ownerRef of secret with pipelineRun, so that it gets cleanedUp with pipelineRun
	if p.pacInfo.SecretAutoCreation {
		err := p.k8int.UpdateSecretWithOwnerRef(ctx, p.logger, pr.Namespace, gitAuthSecretName, pr)
		if err != nil {
			// we still return the created PR with error, and allow caller to decide what to do with the PR, and avoid
			// unneeded SIGSEGV's
			return pr, fmt.Errorf("cannot update pipelinerun %s with ownerRef: %w", pr.GetGenerateName(), err)
		}
	}

	// Create status with the log url
	p.logger.Infof("pipelinerun %s has been created in namespace %s for SHA: %s Target Branch: %s",
		pr.GetName(), match.Repo.GetNamespace(), p.event.SHA, p.event.BaseBranch)

	consoleURL := p.run.Clients.ConsoleUI().DetailURL(pr)
	mt := formatting.MessageTemplate{
		PipelineRunName: pr.GetName(),
		Namespace:       match.Repo.GetNamespace(),
		ConsoleName:     p.run.Clients.ConsoleUI().GetName(),
		ConsoleURL:      consoleURL,
		TknBinary:       settings.TknBinaryName,
		TknBinaryURL:    settings.TknBinaryURL,
	}

	msg, err := mt.MakeTemplate(p.vcx.GetTemplate(provider.StartingPipelineType))
	if err != nil {
		return nil, fmt.Errorf("cannot create message template: %w", err)
	}
	status := provider.StatusOpts{
		Status:                  inProgressStatus,
		Conclusion:              pendingConclusion,
		Text:                    msg,
		DetailsURL:              consoleURL,
		PipelineRunName:         pr.GetName(),
		PipelineRun:             pr,
		OriginalPipelineRunName: pr.GetAnnotations()[keys.OriginalPRName],
	}

	// if pipelineRun is in pending state then report status as queued
	if pr.Spec.Status == tektonv1.PipelineRunSpecStatusPending {
		status.Status = queuedStatus
		if status.Text, err = mt.MakeTemplate(p.vcx.GetTemplate(provider.QueueingPipelineType)); err != nil {
			return nil, fmt.Errorf("cannot create message template: %w", err)
		}
	}

	if err := p.vcx.CreateStatus(ctx, p.event, status); err != nil {
		// we still return the created PR with error, and allow caller to decide what to do with the PR, and avoid
		// unneeded SIGSEGV's
		return pr, fmt.Errorf("cannot use the API on the provider platform to create a in_progress status: %w", err)
	}

	// Patch pipelineRun with logURL annotation, skips for GitHub App as we patch logURL while patching CheckrunID
	if _, ok := pr.Annotations[keys.InstallationID]; !ok {
		pr, err = action.PatchPipelineRun(ctx, p.logger, "logURL", p.run.Clients.Tekton, pr, getLogURLMergePatch(p.run.Clients, pr))
		if err != nil {
			// we still return the created PR with error, and allow caller to decide what to do with the PR, and avoid
			// unneeded SIGSEGV's
			return pr, fmt.Errorf("cannot patch pipelinerun %s: %w", pr.GetGenerateName(), err)
		}
	}

	return pr, nil
}

func getLogURLMergePatch(clients clients.Clients, pr *tektonv1.PipelineRun) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				keys.LogURL: clients.ConsoleUI().DetailURL(pr),
			},
		},
	}
}

func getExecutionOrderPatch(order string) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				keys.ExecutionOrder: order,
			},
		},
	}
}
