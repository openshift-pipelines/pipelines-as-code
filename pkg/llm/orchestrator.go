package llm

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/kubeinteraction"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

// Orchestrator coordinates the complete LLM analysis workflow.
type Orchestrator struct {
	run           *params.Run
	kinteract     kubeinteraction.Interface
	logger        *zap.SugaredLogger
	outputHandler *OutputHandler
}

// NewOrchestrator creates a new LLM analysis orchestrator.
func NewOrchestrator(run *params.Run, kinteract kubeinteraction.Interface, logger *zap.SugaredLogger) *Orchestrator {
	return &Orchestrator{
		run:           run,
		kinteract:     kinteract,
		logger:        logger,
		outputHandler: NewOutputHandler(run, logger),
	}
}

// ExecuteAnalysis performs the complete LLM analysis workflow.
func (o *Orchestrator) ExecuteAnalysis(
	ctx context.Context,
	repo *v1alpha1.Repository,
	pr *tektonv1.PipelineRun,
	event *info.Event,
	prov provider.Interface,
) error {
	if repo.Spec.Settings == nil || repo.Spec.Settings.AIAnalysis == nil || !repo.Spec.Settings.AIAnalysis.Enabled {
		o.logger.Debug("AI analysis not configured or disabled, skipping")
		return nil
	}

	o.logger.Infof("Starting LLM analysis for pipeline %s/%s", pr.Namespace, pr.Name)

	// Create LLM analyzer
	analyzer := NewAnalyzer(o.run, o.kinteract, o.logger)

	// Create analysis request
	request := &AnalyzeRequest{
		PipelineRun: pr,
		Event:       event,
		Repository:  repo,
		Provider:    prov,
	}

	// Perform analysis
	results, err := analyzer.Analyze(ctx, request)
	if err != nil {
		return fmt.Errorf("LLM analysis failed: %w", err)
	}

	if len(results) == 0 {
		o.logger.Debug("No analysis results generated")
		return nil
	}

	// Process analysis results
	for _, result := range results {
		if result.Error != nil {
			o.logger.Warnf("Analysis failed for role %s: %v", result.Role, result.Error)
			continue
		}

		if result.Response == nil {
			o.logger.Warnf("No response for role %s", result.Role)
			continue
		}

		o.logger.Infof("Processing LLM analysis result for role %s, tokens used: %d", result.Role, result.Response.TokensUsed)

		if err := o.outputHandler.HandleOutput(ctx, repo, pr, result, event, prov); err != nil {
			o.logger.Warnf("Failed to handle output for role %s: %v", result.Role, err)
			// Continue processing other results even if one fails
		}
	}

	return nil
}
