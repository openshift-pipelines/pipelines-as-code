package kubeinteraction

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
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

func (k Interaction) CleanupPipelines(ctx context.Context, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun,
	maxKeep int) error {
	repoLabel := filepath.Join(pipelinesascode.GroupName, "repository")
	originalPRLabel := filepath.Join(pipelinesascode.GroupName, "original-prname")
	if _, ok := pr.GetLabels()[originalPRLabel]; !ok {
		return fmt.Errorf("generate pipelienrun should have had the %s label for selection set but we could not find"+
			" it",
			originalPRLabel)
	}

	// Select PR by repository and by its true pipelineRun name (not auto generated one)
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		repoLabel, repo.GetName(), originalPRLabel, pr.GetLabels()[originalPRLabel])
	k.Run.Clients.Log.Infof("selecting pipelineruns by labels \"%s\" for deletion", labelSelector)

	pruns, err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}

	sort.Sort(prByCompletionTime(pruns.Items))

	for c, prun := range pruns.Items {
		if prun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() == "Running" {
			// Should we care about resetting the counter?
			// user ask keep me the last 5 pr, there is one running, so we end up
			// keep the last 4 running. I guess the user really want to keep the running one as Kept.
			k.Run.Clients.Log.Infof("skipping cleaning pr: %s since currently running", prun.GetName())
			continue
		}

		if c >= maxKeep {
			k.Run.Clients.Log.Infof("cleaning old PipelineRun: %s", prun.GetName())
			err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).Delete(
				ctx, prun.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return err
}
