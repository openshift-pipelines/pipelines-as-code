package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

var prCount = stats.Float64("pipelines_as_code_pipelinerun_count",
	"number of pipeline runs by pipelines as code",
	stats.UnitDimensionless)

var prDurationCount = stats.Float64("pipelines_as_code_pipelinerun_duration_seconds_sum",
	"number of seconds all pipelineruns completed in by pipelines as code",
	stats.UnitDimensionless)

// Recorder holds keys for metrics.
type Recorder struct {
	initialized     bool
	provider        tag.Key
	eventType       tag.Key
	namespace       tag.Key
	repository      tag.Key
	status          tag.Key
	reason          tag.Key
	ReportingPeriod time.Duration
}

// NewRecorder creates a new metrics recorder instance
// to log the PAC PipelineRun related metrics.
func NewRecorder() (*Recorder, error) {
	r := &Recorder{
		initialized: true,

		// Default to 30s intervals.
		ReportingPeriod: 30 * time.Second,
	}

	provider, err := tag.NewKey("provider")
	if err != nil {
		return nil, err
	}
	r.provider = provider

	eventType, err := tag.NewKey("event-type")
	if err != nil {
		return nil, err
	}
	r.eventType = eventType

	namespace, err := tag.NewKey("namespace")
	if err != nil {
		return nil, err
	}
	r.namespace = namespace

	repository, err := tag.NewKey("repository")
	if err != nil {
		return nil, err
	}
	r.repository = repository

	status, err := tag.NewKey("status")
	if err != nil {
		return nil, err
	}
	r.status = status

	reason, err := tag.NewKey("reason")
	if err != nil {
		return nil, err
	}
	r.reason = reason

	err = view.Register(
		&view.View{
			Description: prCount.Description(),
			Measure:     prCount,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.provider, r.eventType, r.namespace, r.repository},
		},
		&view.View{
			Description: prDurationCount.Description(),
			Measure:     prDurationCount,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{r.namespace, r.repository, r.status, r.reason},
		},
	)
	if err != nil {
		r.initialized = false
		return r, err
	}

	return r, nil
}

// Count logs number of times a pipelinerun is ran for a provider.
func (r *Recorder) Count(provider, event, namespace, repository string) error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for pipeline runs,  failed to initialize the metrics recorder")
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.provider, provider),
		tag.Insert(r.eventType, event),
		tag.Insert(r.namespace, namespace),
		tag.Insert(r.repository, repository),
	)
	if err != nil {
		return err
	}

	metrics.Record(ctx, prCount.M(1))
	return nil
}

// CountPRDuration collects duration taken by a pipelinerun in seconds accumulate them in prDurationCount.
func (r *Recorder) CountPRDuration(namespace, repository, status, reason string, duration time.Duration) error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for pipelineruns, failed to initialize the metrics recorder")
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.namespace, namespace),
		tag.Insert(r.repository, repository),
		tag.Insert(r.status, status),
		tag.Insert(r.reason, reason),
	)
	if err != nil {
		return err
	}

	metrics.Record(ctx, prDurationCount.M(duration.Seconds()))
	return nil
}
