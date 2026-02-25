package queue

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonVersionedClient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
)

type ManagerInterface interface {
	InitQueues(ctx context.Context, tekton tektonVersionedClient.Interface, pac versioned.Interface) error
	RemoveRepository(repo *v1alpha1.Repository)
	QueuedPipelineRuns(repo *v1alpha1.Repository) []string
	RunningPipelineRuns(repo *v1alpha1.Repository) []string
	AddListToRunningQueue(repo *v1alpha1.Repository, list []string) ([]string, error)
	AddToPendingQueue(repo *v1alpha1.Repository, list []string) error
	RemoveFromQueue(repoKey, prKey string) bool
	RemoveAndTakeItemFromQueue(repo *v1alpha1.Repository, run *tektonv1.PipelineRun) string
}

func RepoKey(repo *v1alpha1.Repository) string {
	return fmt.Sprintf("%s/%s", repo.Namespace, repo.Name)
}

func PrKey(run *tektonv1.PipelineRun) string {
	return fmt.Sprintf("%s/%s", run.Namespace, run.Name)
}
