package metrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

var prCount = stats.Float64("pipelines_as_code_pipelinerun_count",
	"number of pipelineruns by pipelines as code",
	stats.UnitDimensionless)

var prDurationCount = stats.Float64("pipelines_as_code_pipelinerun_duration_seconds_sum",
	"number of seconds all pipelineruns completed in by pipelines as code",
	stats.UnitDimensionless)

var runningPRCount = stats.Float64("pipelines_as_code_running_pipelineruns_count",
	"number of running pipelineruns by pipelines as code",
	stats.UnitDimensionless)

var gitProviderAPIRequestCount = stats.Int64(
	"pipelines_as_code_git_provider_api_request_count",
	"number of API requests from pipelines as code to git providers",
	stats.UnitDimensionless,
)

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

var (
	Once           sync.Once
	R              *Recorder
	ErrRegistering error
)

// NewRecorder creates a new metrics recorder instance
// to log the PAC PipelineRun related metrics.
func NewRecorder() (*Recorder, error) {
	Once.Do(func() {
		R = &Recorder{
			initialized: true,

			// Default to 30s intervals.
			ReportingPeriod: 30 * time.Second,
		}

		provider, errRegistering := tag.NewKey("provider")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.provider = provider

		eventType, errRegistering := tag.NewKey("event-type")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.eventType = eventType

		namespace, errRegistering := tag.NewKey("namespace")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.namespace = namespace

		repository, errRegistering := tag.NewKey("repository")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.repository = repository

		status, errRegistering := tag.NewKey("status")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.status = status

		reason, errRegistering := tag.NewKey("reason")
		if errRegistering != nil {
			ErrRegistering = errRegistering
			return
		}
		R.reason = reason

		var (
			prCountView = &view.View{
				Description: prCount.Description(),
				Measure:     prCount,
				Aggregation: view.Count(),
				TagKeys:     []tag.Key{R.provider, R.eventType, R.namespace, R.repository},
			}

			prDurationView = &view.View{
				Description: prDurationCount.Description(),
				Measure:     prDurationCount,
				Aggregation: view.Sum(),
				TagKeys:     []tag.Key{R.namespace, R.repository, R.status, R.reason},
			}

			runningPRView = &view.View{
				Description: runningPRCount.Description(),
				Measure:     runningPRCount,
				Aggregation: view.LastValue(),
				TagKeys:     []tag.Key{R.namespace, R.repository},
			}
			gitProviderAPIRequestView = &view.View{
				Description: gitProviderAPIRequestCount.Description(),
				Measure:     gitProviderAPIRequestCount,
				Aggregation: view.Count(),
				TagKeys:     []tag.Key{R.provider, R.eventType, R.namespace, R.repository},
			}
		)

		view.Unregister(prCountView, prDurationView, runningPRView, gitProviderAPIRequestView)
		errRegistering = view.Register(prCountView, prDurationView, runningPRView, gitProviderAPIRequestView)
		if errRegistering != nil {
			ErrRegistering = errRegistering
			R.initialized = false
			return
		}
	})

	return R, ErrRegistering
}

func (r Recorder) assertInitialized() error {
	if !r.initialized {
		return fmt.Errorf(
			"ignoring the metrics recording for pipelineruns, failed to initialize the metrics recorder")
	}
	return nil
}

// Count logs number of times a pipelinerun is ran for a provider.
func (r *Recorder) Count(provider, event, namespace, repository string) error {
	if err := r.assertInitialized(); err != nil {
		return err
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
	if err := r.assertInitialized(); err != nil {
		return err
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

// RunningPipelineRuns emits the number of running PipelineRuns for a repository and namespace.
func (r *Recorder) RunningPipelineRuns(namespace, repository string, runningPRs float64) error {
	if err := r.assertInitialized(); err != nil {
		return err
	}

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(r.namespace, namespace),
		tag.Insert(r.repository, repository),
	)
	if err != nil {
		return err
	}

	metrics.Record(ctx, runningPRCount.M(runningPRs))
	return nil
}

func (r *Recorder) EmitRunningPRsMetrics(prl []*tektonv1.PipelineRun) error {
	if len(prl) == 0 {
		return nil
	}

	// bifurcate PipelineRuns based on their namespace and repository
	runningPRs := map[string]int{}
	completedPRsKeys := map[string]struct{}{}
	for _, pr := range prl {
		// Check if PipelineRun has Repository annotation it means PR is created by PAC.
		if repository, ok := pr.GetAnnotations()[keys.Repository]; ok {
			key := fmt.Sprintf("%s/%s", pr.GetNamespace(), repository)
			// check if PipelineRun is running.
			if !pr.IsDone() {
				runningPRs[key]++
			} else {
				// add it in completed, and we don't want completed PipelineRuns count.
				completedPRsKeys[key] = struct{}{}
			}
		}
	}

	for k, v := range runningPRs {
		nsKeys := strings.Split(k, "/")
		if err := r.RunningPipelineRuns(nsKeys[0], nsKeys[1], float64(v)); err != nil {
			return err
		}
	}

	// report zero for the keys which aren't in runningPRs.
	for key := range completedPRsKeys {
		// if key isn't there in runningPRs then it should be reported 0
		// otherwise it was reported in previous loop.
		if _, ok := runningPRs[key]; !ok {
			nsKeys := strings.Split(key, "/")
			if err := r.RunningPipelineRuns(nsKeys[0], nsKeys[1], 0); err != nil {
				return err
			}
		}
	}

	return nil
}

// ReportRunningPipelineRuns reports running PipelineRuns on our configured ReportingPeriod
// until the context is cancelled.
func (r *Recorder) ReportRunningPipelineRuns(ctx context.Context, lister listers.PipelineRunLister) {
	logger := logging.FromContext(ctx)

	for {
		delay := time.NewTimer(r.ReportingPeriod)
		select {
		case <-ctx.Done():
			// When the context is cancelled, stop reporting.
			if !delay.Stop() {
				<-delay.C
			}
			return

		case <-delay.C:
			prl, err := lister.List(labels.Everything())
			if err != nil {
				logger.Warnf("Failed to list PipelineRuns : %v", err)
				continue
			}
			// Every 30s surface a metric for the number of running pipelines.
			if err := r.EmitRunningPRsMetrics(prl); err != nil {
				logger.Warnf("Failed to log the metrics : %v", err)
			}
		}
	}
}

func (r *Recorder) ReportGitProviderAPIUsage(provider, event, namespace, repository string) error {
	if err := r.assertInitialized(); err != nil {
		return err
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

	metrics.Record(ctx, gitProviderAPIRequestCount.M(1))
	return nil
}

func ResetRecorder() {
	Once = sync.Once{}
	R = nil
	ErrRegistering = nil
}
