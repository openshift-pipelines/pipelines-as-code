package pipelineascode

import (
	"context"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/typed/pipelinesascode/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineAsCode struct {
	Client pacclient.PipelinesascodeV1alpha1Interface
}

func (p PipelineAsCode) FilterBy(url, branch, eventType string) (apipac.Repository, error) {
	var repository apipac.Repository
	repositories, err := p.Client.Repositories("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return repository, err
	}
	for _, value := range repositories.Items {
		if value.Spec.URL == url && value.Spec.Branch == branch && value.Spec.EventType == eventType {
			return value, nil
		}
	}
	return repository, nil
}

/// PipelineRunHasFailed return status of PR  success failed or skipped
func (p PipelineAsCode) PipelineRunHasFailed(pr *tektonv1beta1.PipelineRun) string {
	if len(pr.Status.Conditions) == 0 {
		return "neutral"
	}
	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return "failure"
	}
	return "success"
}
