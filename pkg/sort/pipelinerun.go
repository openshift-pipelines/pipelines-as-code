package sort

import (
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// From tekton cli prsort package
type prSortByCompletionTime []v1beta1.PipelineRun

func (prs prSortByCompletionTime) Len() int      { return len(prs) }
func (prs prSortByCompletionTime) Swap(i, j int) { prs[i], prs[j] = prs[j], prs[i] }
func (prs prSortByCompletionTime) Less(i, j int) bool {
	if prs[j].Status.CompletionTime == nil {
		return false
	}
	if prs[i].Status.CompletionTime == nil {
		return true
	}
	return prs[j].Status.CompletionTime.Before(prs[i].Status.CompletionTime)
}

func PipelineRunSortByCompletionTime(items []v1beta1.PipelineRun) []v1beta1.PipelineRun {
	sort.Sort(prSortByCompletionTime(items))
	return items
}

func PipelineRunSortByStartTime(prs []v1beta1.PipelineRun) {
	sort.Sort(byStartTime(prs))
}

type byStartTime []v1beta1.PipelineRun

func (prs byStartTime) Len() int      { return len(prs) }
func (prs byStartTime) Swap(i, j int) { prs[i], prs[j] = prs[j], prs[i] }
func (prs byStartTime) Less(i, j int) bool {
	if prs[j].Status.StartTime == nil {
		return false
	}
	if prs[i].Status.StartTime == nil {
		return true
	}
	return prs[j].Status.StartTime.Before(prs[i].Status.StartTime)
}
