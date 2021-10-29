package sort

import (
	"sort"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
)

type repoSortRunStatus []v1alpha1.RepositoryRunStatus

func (rs repoSortRunStatus) Len() int {
	return len(rs)
}

func (rs repoSortRunStatus) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs repoSortRunStatus) Less(i, j int) bool {
	if rs[j].StartTime == nil {
		return false
	}

	if rs[i].StartTime == nil {
		return true
	}

	return rs[j].StartTime.Before(rs[i].StartTime)
}

func RepositorySortRunStatus(repoStatus []v1alpha1.RepositoryRunStatus) []v1alpha1.RepositoryRunStatus {
	rrstatus := repoSortRunStatus{}
	for _, status := range repoStatus {
		rrstatus = append(rrstatus, status)
	}
	sort.Sort(rrstatus)
	return rrstatus
}
