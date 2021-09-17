package kubeinteraction

import (
	"context"
	"fmt"
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// From tekton cli prsort package
type prByCompletionTime []v1beta1.PipelineRun

func (prs prByCompletionTime) Len() int      { return len(prs) }
func (prs prByCompletionTime) Swap(i, j int) { prs[i], prs[j] = prs[j], prs[i] }
func (prs prByCompletionTime) Less(i, j int) bool {
	if prs[j].Status.CompletionTime == nil {
		return false
	}
	if prs[i].Status.CompletionTime == nil {
		return true
	}
	return prs[j].Status.CompletionTime.Before(prs[i].Status.CompletionTime)
}

func (k Interaction) CleanupPipelines(ctx context.Context, namespace string, repositoryName string, maxKeep int) error {
	labelSelector := fmt.Sprintf("pipelinesascode.tekton.dev/repository=%s", repositoryName)

	pruns, err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	sort.Sort(prByCompletionTime(pruns.Items))

	for c, v := range pruns.Items {
		if v.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() == "Running" {
			k.Run.Clients.Log.Infof("Skipping Cleanining up %s since it is currently Running", v.GetName())
			continue
		}
		if c >= maxKeep {
			k.Run.Clients.Log.Infof("Cleaning old PipelineRun: %s", v.GetName())
			err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(namespace).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return err
}
