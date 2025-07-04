package metrics

import (
	"context"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/pkg/metrics"
)

var (
	// QueueValidationErrors tracks the number of queue validation errors per repository.
	queueValidationErrors = stats.Int64(
		"pac_queue_validation_errors_total",
		"Total number of queue validation errors per repository",
		stats.UnitDimensionless,
	)

	// QueueValidationWarnings tracks the number of queue validation warnings per repository.
	queueValidationWarnings = stats.Int64(
		"pac_queue_validation_warnings_total",
		"Total number of queue validation warnings per repository",
		stats.UnitDimensionless,
	)

	// QueueRepairOperations tracks the number of queue repair operations.
	queueRepairOperations = stats.Int64(
		"pac_queue_repair_operations_total",
		"Total number of queue repair operations",
		stats.UnitDimensionless,
	)

	// QueueState tracks the current state of queues.
	queueState = stats.Int64(
		"pac_queue_state",
		"Current state of concurrency queues",
		stats.UnitDimensionless,
	)

	// QueueUtilization tracks queue utilization percentage.
	queueUtilization = stats.Float64(
		"pac_queue_utilization_percentage",
		"Queue utilization as a percentage of concurrency limit",
		stats.UnitDimensionless,
	)

	// QueueRecoveryTime tracks how long queue recovery takes.
	queueRecoveryTime = stats.Float64(
		"pac_queue_recovery_duration_seconds",
		"Time taken to recover queue state",
		stats.UnitSeconds,
	)
)

// QueueMetricsRecorder records metrics related to queue operations.
type QueueMetricsRecorder struct {
	logger     *zap.SugaredLogger
	repository tag.Key
	namespace  tag.Key
	state      tag.Key
	status     tag.Key
}

// NewQueueMetricsRecorder creates a new queue metrics recorder.
func NewQueueMetricsRecorder(logger *zap.SugaredLogger) (*QueueMetricsRecorder, error) {
	repository, err := tag.NewKey("repository")
	if err != nil {
		return nil, err
	}

	namespace, err := tag.NewKey("namespace")
	if err != nil {
		return nil, err
	}

	state, err := tag.NewKey("state")
	if err != nil {
		return nil, err
	}

	status, err := tag.NewKey("status")
	if err != nil {
		return nil, err
	}

	recorder := &QueueMetricsRecorder{
		logger:     logger,
		repository: repository,
		namespace:  namespace,
		state:      state,
		status:     status,
	}

	// Register views
	views := []*view.View{
		{
			Description: queueValidationErrors.Description(),
			Measure:     queueValidationErrors,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{repository, namespace},
		},
		{
			Description: queueValidationWarnings.Description(),
			Measure:     queueValidationWarnings,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{repository, namespace},
		},
		{
			Description: queueRepairOperations.Description(),
			Measure:     queueRepairOperations,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{repository, namespace, status},
		},
		{
			Description: queueState.Description(),
			Measure:     queueState,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{repository, namespace, state},
		},
		{
			Description: queueUtilization.Description(),
			Measure:     queueUtilization,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{repository, namespace},
		},
		{
			Description: queueRecoveryTime.Description(),
			Measure:     queueRecoveryTime,
			Aggregation: view.Distribution(0.1, 0.5, 1, 2, 5, 10, 30, 60),
			TagKeys:     []tag.Key{repository, namespace},
		},
	}

	view.Unregister(views...)
	if err := view.Register(views...); err != nil {
		return nil, err
	}

	return recorder, nil
}

// RecordQueueValidation records metrics for queue validation results.
func (r *QueueMetricsRecorder) RecordQueueValidation(results []sync.QueueValidationResult) {
	for _, result := range results {
		// Parse repository key (format: namespace/name)
		repoParts := parseRepoKey(result.RepositoryKey)
		if len(repoParts) != 2 {
			r.logger.Warnf("Invalid repository key format: %s", result.RepositoryKey)
			continue
		}
		namespace, repoName := repoParts[0], repoParts[1]

		ctx, err := tag.New(
			context.Background(),
			tag.Insert(r.repository, repoName),
			tag.Insert(r.namespace, namespace),
		)
		if err != nil {
			r.logger.Errorf("Failed to create context for metrics: %v", err)
			continue
		}

		// Record validation errors
		metrics.Record(ctx, queueValidationErrors.M(int64(len(result.Errors))))

		// Record validation warnings
		metrics.Record(ctx, queueValidationWarnings.M(int64(len(result.Warnings))))

		// Record queue state
		ctxRunning, _ := tag.New(ctx, tag.Insert(r.state, "running"))
		metrics.Record(ctxRunning, queueState.M(int64(result.RunningCount)))

		ctxPending, _ := tag.New(ctx, tag.Insert(r.state, "pending"))
		metrics.Record(ctxPending, queueState.M(int64(result.PendingCount)))

		// Record queue utilization
		if result.ExpectedCount > 0 {
			utilization := float64(result.RunningCount) / float64(result.ExpectedCount) * 100
			metrics.Record(ctx, queueUtilization.M(utilization))
		}
	}
}

// RecordQueueRepair records metrics for queue repair operations.
func (r *QueueMetricsRecorder) RecordQueueRepair(repoKey, status string) {
	repoParts := parseRepoKey(repoKey)
	if len(repoParts) != 2 {
		r.logger.Warnf("Invalid repository key format: %s", repoKey)
		return
	}
	namespace, repoName := repoParts[0], repoParts[1]

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.repository, repoName),
		tag.Insert(r.namespace, namespace),
		tag.Insert(r.status, status),
	)
	if err != nil {
		r.logger.Errorf("Failed to create context for metrics: %v", err)
		return
	}

	metrics.Record(ctx, queueRepairOperations.M(1))
}

// RecordQueueRecoveryTime records the time taken for queue recovery.
func (r *QueueMetricsRecorder) RecordQueueRecoveryTime(repoKey string, duration time.Duration) {
	repoParts := parseRepoKey(repoKey)
	if len(repoParts) != 2 {
		r.logger.Warnf("Invalid repository key format: %s", repoKey)
		return
	}
	namespace, repoName := repoParts[0], repoParts[1]

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.repository, repoName),
		tag.Insert(r.namespace, namespace),
	)
	if err != nil {
		r.logger.Errorf("Failed to create context for metrics: %v", err)
		return
	}

	metrics.Record(ctx, queueRecoveryTime.M(duration.Seconds()))
}

// parseRepoKey parses a repository key in the format "namespace/name".
func parseRepoKey(repoKey string) []string {
	// This is a simple implementation - in practice, you might want more robust parsing
	// that handles edge cases like repository names containing "/"
	parts := make([]string, 0, 2)
	lastSlash := -1
	for i, char := range repoKey {
		if char == '/' {
			if lastSlash == -1 {
				parts = append(parts, repoKey[:i])
				lastSlash = i
			}
		}
	}
	if lastSlash != -1 {
		parts = append(parts, repoKey[lastSlash+1:])
	}
	return parts
}
