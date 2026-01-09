package status

import (
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

type Conclusion string

const (
	//nolint:misspell
	// NOTE: some VCS systems use "canceled" with one "l".
	ConclusionCancelled Conclusion = "cancelled"
	ConclusionCompleted Conclusion = "completed"
	ConclusionFailure   Conclusion = "failure"
	ConclusionNeutral   Conclusion = "neutral"
	ConclusionPending   Conclusion = "pending"
	ConclusionSkipped   Conclusion = "skipped"
	ConclusionSuccess   Conclusion = "success"
)

//nolint:revive
type StatusOpts struct {
	PipelineRun              *v1.PipelineRun
	PipelineRunName          string
	OriginalPipelineRunName  string
	Status                   string
	Conclusion               Conclusion
	Text                     string
	DetailsURL               string
	Summary                  string
	Title                    string
	InstanceCountForCheckRun int
	AccessDenied             bool
}
