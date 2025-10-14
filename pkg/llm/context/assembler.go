package context

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	kstatus "github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction/status"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

const (
	// DefaultMaxLogLines is the default maximum number of log lines to include in context.
	DefaultMaxLogLines = 50
)

// Assembler builds context data for LLM analysis from pipeline and event information.
type Assembler struct {
	run       *params.Run
	kinteract kubeinteraction.Interface
	logger    *zap.SugaredLogger
}

// NewAssembler creates a new context assembler.
func NewAssembler(run *params.Run, kinteract kubeinteraction.Interface, logger *zap.SugaredLogger) *Assembler {
	return &Assembler{
		run:       run,
		kinteract: kinteract,
		logger:    logger,
	}
}

// BuildContext assembles context data based on the provided configuration.
func (a *Assembler) BuildContext(
	ctx context.Context,
	pipelineRun *tektonv1.PipelineRun,
	event *info.Event,
	contextConfig *v1alpha1.ContextConfig,
	provider provider.Interface,
) (map[string]any, error) {
	contextData := make(map[string]any)

	if contextConfig == nil {
		// Return basic pipeline information if no specific context is requested
		return a.buildBasicPipelineContext(pipelineRun, event), nil
	}

	// Add commit content if requested
	if contextConfig.CommitContent {
		if commitData, err := a.buildCommitContent(ctx, event, provider); err != nil {
			a.logger.Warnf("Failed to build commit content: %v", err)
		} else {
			contextData["commit"] = commitData
		}
	}

	// Add PR content if requested
	if contextConfig.PRContent {
		if prData, err := a.buildPRContent(ctx, event, provider); err != nil {
			a.logger.Warnf("Failed to build PR content: %v", err)
		} else {
			contextData["pull_request"] = prData
		}
	}

	// Add error content if requested
	if contextConfig.ErrorContent {
		if errorData := a.buildErrorContent(ctx, pipelineRun); errorData != nil {
			contextData["errors"] = errorData
		}
	}

	// Add container logs if requested
	if contextConfig.ContainerLogs != nil && contextConfig.ContainerLogs.Enabled {
		maxLines := contextConfig.ContainerLogs.MaxLines
		if maxLines == 0 {
			maxLines = DefaultMaxLogLines
		}
		if logData := a.buildContainerLogs(ctx, pipelineRun, maxLines); logData != nil {
			contextData["logs"] = logData
		}
	}

	// Always include basic pipeline information
	contextData["pipeline"] = a.buildBasicPipelineContext(pipelineRun, event)

	return contextData, nil
}

// buildBasicPipelineContext creates basic pipeline context information.
func (a *Assembler) buildBasicPipelineContext(pipelineRun *tektonv1.PipelineRun, event *info.Event) map[string]any {
	pipelineData := map[string]any{
		"name":      pipelineRun.Name,
		"namespace": pipelineRun.Namespace,
		"status":    "unknown",
	}

	// Add status information if available
	if len(pipelineRun.Status.Conditions) > 0 {
		condition := pipelineRun.Status.Conditions[0]
		pipelineData["status"] = condition.Status
		pipelineData["reason"] = condition.Reason
		pipelineData["message"] = condition.Message
	}

	// Add timing information
	if pipelineRun.Status.StartTime != nil {
		pipelineData["start_time"] = pipelineRun.Status.StartTime.Time
	}
	if pipelineRun.Status.CompletionTime != nil {
		pipelineData["completion_time"] = pipelineRun.Status.CompletionTime.Time
	}

	// Add event information
	if event != nil {
		pipelineData["event_type"] = event.EventType
		pipelineData["sha"] = event.SHA
		pipelineData["base_branch"] = event.BaseBranch
		pipelineData["head_branch"] = event.HeadBranch
	}

	return pipelineData
}

// buildCommitContent builds commit-related context information.
func (a *Assembler) buildCommitContent(ctx context.Context, event *info.Event, provider provider.Interface) (map[string]any, error) {
	if event == nil {
		return nil, fmt.Errorf("event is nil")
	}

	commitData := map[string]any{
		"sha":     event.SHA,
		"message": event.SHATitle,
	}

	// Try to get additional commit information from the provider
	if provider != nil {
		if err := provider.GetCommitInfo(ctx, event); err != nil {
			a.logger.Warnf("Failed to get additional commit info: %v", err)
		}
	}

	return commitData, nil
}

// buildPRContent builds pull request context information.
func (a *Assembler) buildPRContent(_ context.Context, event *info.Event, _ provider.Interface) (map[string]any, error) {
	if event == nil || event.PullRequestNumber == 0 {
		return nil, fmt.Errorf("no pull request information available")
	}

	prData := map[string]any{
		"number":      event.PullRequestNumber,
		"title":       event.PullRequestTitle,
		"head_branch": event.HeadBranch,
		"base_branch": event.BaseBranch,
	}

	// Note: PR body is not available in the Event struct

	return prData, nil
}

// buildErrorContent builds error and failure context information.
func (a *Assembler) buildErrorContent(ctx context.Context, pipelineRun *tektonv1.PipelineRun) map[string]any {
	// Check if pipeline failed
	if len(pipelineRun.Status.Conditions) == 0 {
		return nil
	}

	condition := pipelineRun.Status.Conditions[0]
	if condition.Status != "False" {
		return nil
	}

	errorData := map[string]any{
		"condition_reason":  condition.Reason,
		"condition_message": condition.Message,
	}

	// Get detailed task failure information
	taskInfos := kstatus.CollectFailedTasksLogSnippet(ctx, a.run, a.kinteract, pipelineRun, 3)
	if len(taskInfos) > 0 {
		sortedTaskInfos := sort.TaskInfos(taskInfos)

		var failedTasks []map[string]any
		for _, taskInfo := range sortedTaskInfos {
			failedTask := map[string]any{
				"name":        taskInfo.Name,
				"reason":      taskInfo.Reason,
				"message":     taskInfo.Message,
				"log_snippet": taskInfo.LogSnippet,
			}

			if taskInfo.DisplayName != "" {
				failedTask["display_name"] = taskInfo.DisplayName
			}

			if taskInfo.CompletionTime != nil {
				failedTask["completion_time"] = taskInfo.CompletionTime.Time
			}

			failedTasks = append(failedTasks, failedTask)
		}

		errorData["failed_tasks"] = failedTasks
	}

	return errorData
}

// buildContainerLogs builds container logs context information.
func (a *Assembler) buildContainerLogs(ctx context.Context, pipelineRun *tektonv1.PipelineRun, maxLines int) map[string]any {
	// Get detailed task information with logs
	taskInfos := kstatus.CollectFailedTasksLogSnippet(ctx, a.run, a.kinteract, pipelineRun, int64(maxLines))
	if len(taskInfos) == 0 {
		return nil
	}

	logs := []map[string]any{}
	for _, taskInfo := range taskInfos {
		logEntry := map[string]any{
			"task_name": taskInfo.Name,
			"log_lines": strings.Split(taskInfo.LogSnippet, "\n"),
		}

		if taskInfo.DisplayName != "" {
			logEntry["display_name"] = taskInfo.DisplayName
		}

		logs = append(logs, logEntry)
	}

	return map[string]any{
		"failed_tasks": logs,
		"max_lines":    maxLines,
	}
}

// BuildCELContext builds context data for CEL expression evaluation.
func (a *Assembler) BuildCELContext(
	pipelineRun *tektonv1.PipelineRun,
	event *info.Event,
	repo *v1alpha1.Repository,
) (map[string]any, error) {
	// Convert PipelineRun to map for CEL access
	prMap, err := a.pipelineRunToMap(pipelineRun)
	if err != nil {
		return nil, fmt.Errorf("failed to convert PipelineRun to map: %w", err)
	}

	// Convert Repository to map for CEL access
	repoMap, err := a.repositoryToMap(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Repository to map: %w", err)
	}

	celData := map[string]any{
		"body": map[string]any{
			"pipelineRun": prMap,
			"repository":  repoMap,
		},
		"pac": make(map[string]string), // PAC parameters will be populated by caller
	}

	// Add event information to CEL context
	if event != nil {
		eventMap := map[string]any{
			"event_type":   event.EventType,
			"sha":          event.SHA,
			"base_branch":  event.BaseBranch,
			"head_branch":  event.HeadBranch,
			"organization": event.Organization,
			"repository":   event.Repository,
		}

		if event.PullRequestNumber > 0 {
			eventMap["pull_request_number"] = event.PullRequestNumber
			eventMap["pull_request_title"] = event.PullRequestTitle
		}

		if bodyMap, ok := celData["body"].(map[string]any); ok {
			bodyMap["event"] = eventMap
		}
	}

	return celData, nil
}

// pipelineRunToMap converts a PipelineRun to a map for CEL access.
func (a *Assembler) pipelineRunToMap(pr *tektonv1.PipelineRun) (map[string]any, error) {
	// Marshal to JSON and back to get a clean map representation
	jsonData, err := json.Marshal(pr)
	if err != nil {
		return nil, err
	}

	var prMap map[string]any
	if err := json.Unmarshal(jsonData, &prMap); err != nil {
		return nil, err
	}

	return prMap, nil
}

// repositoryToMap converts a Repository to a map for CEL access.
func (a *Assembler) repositoryToMap(repo *v1alpha1.Repository) (map[string]any, error) {
	// Marshal to JSON and back to get a clean map representation
	jsonData, err := json.Marshal(repo)
	if err != nil {
		return nil, err
	}

	var repoMap map[string]any
	if err := json.Unmarshal(jsonData, &repoMap); err != nil {
		return nil, err
	}

	return repoMap, nil
}
