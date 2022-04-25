package kubeinteraction

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	psort "github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func (k Interaction) CleanupPipelines(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *v1beta1.PipelineRun, maxKeep int) error {
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
	logger.Infof("selecting pipelineruns by labels \"%s\" for deletion", labelSelector)

	pruns, err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).List(ctx,
		metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}

	for c, prun := range psort.PipelineRunSortByCompletionTime(pruns.Items) {
		if prun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() == "Running" {
			logger.Infof("skipping %s since currently running", prun.GetName())
			continue
		}

		if c >= maxKeep {
			logger.Infof("cleaning old PipelineRun: %s", prun.GetName())
			err := k.Run.Clients.Tekton.TektonV1beta1().PipelineRuns(repo.GetNamespace()).Delete(
				ctx, prun.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
