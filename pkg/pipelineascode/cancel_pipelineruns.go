package pipelineascode

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/action"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var cancelMergePatch = map[string]interface{}{
	"spec": map[string]interface{}{
		"status": tektonv1.PipelineRunSpecStatusCancelledRunFinally,
	},
}

// cancelInProgress cancels all PipelineRuns associated with a given repository and pull request,
// except for the one that triggered the cancellation. It first checks if the cancellation is in progress
// and if the repository has a concurrency limit. If a concurrency limit is set, it returns an error as
// cancellation is not supported with concurrency limits. It then retrieves the original pull request name
// from the annotations and lists all PipelineRuns with matching labels. For each PipelineRun that is not
// already done, cancelled, or gracefully stopped, it patches the PipelineRun to cancel it.
func (p *PacRun) cancelInProgress(ctx context.Context, matchPR *tektonv1.PipelineRun, repo *v1alpha1.Repository) error {
	if matchPR == nil {
		return nil
	}
	if key, ok := matchPR.GetAnnotations()[keys.CancelInProgress]; !ok || key != "true" {
		return nil
	}

	if repo.Spec.ConcurrencyLimit != nil && *repo.Spec.ConcurrencyLimit > 0 {
		return fmt.Errorf("cancel in progress is not supported with concurrency limit")
	}

	prName, ok := matchPR.GetAnnotations()[keys.OriginalPRName]
	if !ok {
		return nil
	}
	labelSelector := getLabelSelector(map[string]string{
		keys.URLRepository:  formatting.CleanValueKubernetes(p.event.Repository),
		keys.OriginalPRName: prName,
	})

	prs, err := p.run.Clients.Tekton.TektonV1().PipelineRuns(matchPR.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pipelineRuns : %w", err)
	}
	var wg sync.WaitGroup
	for _, pr := range prs.Items {
		if pr.GetName() == matchPR.GetName() {
			continue
		}
		if pr.IsDone() {
			continue
		}
		if pr.IsCancelled() || pr.IsGracefullyCancelled() || pr.IsGracefullyStopped() {
			continue
		}

		p.logger.Infof("cancel-in-progress: cancelling pipelinerun %v/%v", pr.GetNamespace(), pr.GetName())
		wg.Add(1)
		go func(ctx context.Context, pr tektonv1.PipelineRun) {
			defer wg.Done()
			if _, err := action.PatchPipelineRun(ctx, p.logger, "cancel patch", p.run.Clients.Tekton, &pr, cancelMergePatch); err != nil {
				errMsg := fmt.Sprintf("failed to cancel pipelineRun %s/%s: %s", pr.GetNamespace(), pr.GetName(), err.Error())
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", errMsg)
			}
		}(ctx, pr)
	}
	wg.Wait()

	return nil
}

func (p *PacRun) cancelPipelineRuns(ctx context.Context, repo *v1alpha1.Repository) error {
	labelSelector := getLabelSelector(map[string]string{
		keys.URLRepository: formatting.CleanValueKubernetes(p.event.Repository),
		keys.SHA:           formatting.CleanValueKubernetes(p.event.SHA),
	})

	if p.event.TriggerTarget == triggertype.PullRequest {
		labelSelector = getLabelSelector(map[string]string{
			keys.PullRequest: strconv.Itoa(p.event.PullRequestNumber),
		})
	}

	prs, err := p.run.Clients.Tekton.TektonV1().PipelineRuns(repo.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pipelineRuns : %w", err)
	}

	if len(prs.Items) == 0 {
		msg := fmt.Sprintf("no pipelinerun found for repository: %v , sha: %v and pulRequest %v",
			p.event.Repository, p.event.SHA, p.event.PullRequestNumber)
		p.eventEmitter.EmitMessage(repo, zap.InfoLevel, "RepositoryPipelineRun", msg)
		return nil
	}

	var wg sync.WaitGroup
	for _, pr := range prs.Items {
		if p.event.TargetCancelPipelineRun != "" {
			if prName, ok := pr.GetAnnotations()[keys.OriginalPRName]; !ok || prName != p.event.TargetCancelPipelineRun {
				continue
			}
		}
		if pr.IsDone() {
			p.logger.Infof("pipelinerun %v/%v is done, skipping cancellation", pr.GetNamespace(), pr.GetName())
			continue
		}
		if pr.IsCancelled() || pr.IsGracefullyCancelled() || pr.IsGracefullyStopped() {
			p.logger.Infof("pipelinerun %v/%v is already in %v state", pr.GetNamespace(), pr.GetName(), pr.Spec.Status)
			continue
		}

		wg.Add(1)
		go func(ctx context.Context, pr tektonv1.PipelineRun) {
			defer wg.Done()
			if _, err := action.PatchPipelineRun(ctx, p.logger, "cancel patch", p.run.Clients.Tekton, &pr, cancelMergePatch); err != nil {
				errMsg := fmt.Sprintf("failed to cancel pipelineRun %s/%s: %s", pr.GetNamespace(), pr.GetName(), err.Error())
				p.eventEmitter.EmitMessage(repo, zap.ErrorLevel, "RepositoryPipelineRun", errMsg)
			}
		}(ctx, pr)
	}
	wg.Wait()

	return nil
}

func getLabelSelector(labelsMap map[string]string) string {
	labelSelector := labels.NewSelector()
	for k, v := range labelsMap {
		req, _ := labels.NewRequirement(k, selection.Equals, []string{v})
		if req != nil {
			labelSelector = labelSelector.Add(*req)
		}
	}
	return labelSelector.String()
}
