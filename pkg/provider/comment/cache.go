// Package comment provides shared utilities for comment management across providers.
package comment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/action"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	psort "github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	versioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CacheCommentID stores the comment ID in the PipelineRun annotation for future updates.
// This enables fast comment updates without listing all PR comments.
func CacheCommentID(ctx context.Context, logger *zap.SugaredLogger, tektonClient versioned.Interface, pr *tektonv1.PipelineRun, pipelineName string, commentID int64) error {
	if tektonClient == nil {
		return fmt.Errorf("kubernetes client not available")
	}

	commentIDKey := fmt.Sprintf("%s-%s", keys.StatusCommentID, pipelineName)
	annotations := map[string]string{
		commentIDKey: strconv.FormatInt(commentID, 10),
	}

	if _, err := action.PatchPipelineRun(ctx, logger, "cache comment ID", tektonClient, pr, map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}); err != nil {
		return fmt.Errorf("failed to patch pipelinerun: %w", err)
	}

	logger.Debugf("Cached comment ID %d for pipeline %s", commentID, pipelineName)
	return nil
}

// GetCachedCommentID retrieves the cached comment ID from PipelineRun annotations.
// It first checks the current PipelineRun, then searches sibling PipelineRuns
// with the same OriginalPRName label AND same Pull Request number, sorted by completion time (newest first).
// Returns 0 if no cached ID exists.
func GetCachedCommentID(ctx context.Context, logger *zap.SugaredLogger, tektonClient versioned.Interface, pr *tektonv1.PipelineRun, namespace, pipelineName string, pullRequestNumber int) int64 {
	commentIDKey := fmt.Sprintf("%s-%s", keys.StatusCommentID, pipelineName)

	// Check the current PipelineRun's annotation
	if pr != nil {
		if commentIDStr, ok := pr.GetAnnotations()[commentIDKey]; ok {
			if commentID, err := strconv.ParseInt(commentIDStr, 10, 64); err == nil {
				logger.Debugf("Found comment ID %d in current PipelineRun %s", commentID, pr.GetName())
				return commentID
			}
		}
	}

	if tektonClient == nil {
		return 0
	}

	if namespace == "" && pr != nil {
		namespace = pr.GetNamespace()
	}
	if namespace == "" {
		return 0
	}

	// Filter by both pipeline name AND pull request number to avoid cross-PR cache hits
	labelSelector := fmt.Sprintf(
		"%s=%s,%s=%d",
		keys.OriginalPRName, formatting.CleanValueKubernetes(pipelineName),
		keys.PullRequest, pullRequestNumber,
	)

	pruns, err := tektonClient.TektonV1().PipelineRuns(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		logger.Warnf("Failed to list PipelineRuns for comment ID lookup: %v", err)
		return 0
	}

	// Get the most recent cached comment ID
	psort.PipelineRunSortByCompletionTime(pruns.Items)

	for i := range pruns.Items {
		prun := &pruns.Items[i]
		// Skip the current PipelineRun, already checked
		if pr != nil && prun.GetName() == pr.GetName() {
			continue
		}
		if commentIDStr, ok := prun.GetAnnotations()[commentIDKey]; ok {
			if commentID, err := strconv.ParseInt(commentIDStr, 10, 64); err == nil {
				logger.Debugf("Found comment ID %d in PipelineRun %s", commentID, prun.GetName())
				return commentID
			}
		}
	}

	return 0
}
