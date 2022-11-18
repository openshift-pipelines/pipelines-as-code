package sort

import (
	"sort"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
)

type taskInfoSorter []pacv1alpha1.TaskInfos

func (s taskInfoSorter) Len() int { return len(s) }

func (s taskInfoSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s taskInfoSorter) Less(i, j int) bool {
	return s[i].CompletionTime.Before(s[j].CompletionTime)
}

func TaskInfos(taskinfos map[string]pacv1alpha1.TaskInfos) []pacv1alpha1.TaskInfos {
	tis := taskInfoSorter{}
	for _, ti := range taskinfos {
		tis = append(tis, ti)
	}
	sort.Sort(tis)
	return tis
}
