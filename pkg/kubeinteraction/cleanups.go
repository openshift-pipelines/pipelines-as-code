package kubeinteraction

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	psort "github.com/openshift-pipelines/pipelines-as-code/pkg/sort"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func (k Interaction) CleanupPipelines(ctx context.Context, logger *zap.SugaredLogger, repo *v1alpha1.Repository, pr *tektonv1.PipelineRun, maxKeep int) error {
	if _, ok := pr.GetAnnotations()[keys.OriginalPRName]; !ok {
		return fmt.Errorf("generate pipelinerun should have had the %s label for selection set but we could not find"+
			" it",
			keys.OriginalPRName)
	}

	// Select PR by repository and by its true pipelineRun name (not auto generated one)
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		keys.Repository, formatting.CleanValueKubernetes(repo.GetName()), keys.OriginalPRName, formatting.CleanValueKubernetes(pr.GetLabels()[keys.OriginalPRName]))
	logger.Infof("selecting pipelineruns by labels \"%s\" for deletion", labelSelector)

	pruns, err := k.Run.Clients.Tekton.TektonV1().PipelineRuns(repo.GetNamespace()).List(ctx,
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
			logger.With("name", prun.Name).With("action", "DELETE").
				Infof("cleaning old PipelineRun: %s", prun.GetName())
			err := k.Run.Clients.Tekton.TektonV1().PipelineRuns(repo.GetNamespace()).Delete(
				ctx, prun.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
