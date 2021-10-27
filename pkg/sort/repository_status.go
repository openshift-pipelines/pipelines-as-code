package sort

import (
	"sort"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
)

type repoRunStatus []v1alpha1.RepositoryRunStatus

func (rs repoRunStatus) Len() int {
	return len(rs)
}

func (rs repoRunStatus) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func (rs repoRunStatus) Less(i, j int) bool {
	if rs[j].StartTime == nil {
		return false
	}

	if rs[i].StartTime == nil {
		return true
	}

	return rs[j].StartTime.Before(rs[i].StartTime)
}

func RepositoryRunStatus(repoStatus []v1alpha1.RepositoryRunStatus) []v1alpha1.RepositoryRunStatus {
	rrstatus := repoRunStatus{}
	for _, status := range repoStatus {
		rrstatus = append(rrstatus, status)
	}
	sort.Sort(rrstatus)
	return rrstatus
}
