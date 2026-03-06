package concurrency

import (
	"context"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pacVersionedClient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonVersionedClient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
)

type TestQMI struct {
	QueuedPrs    []string
	RunningQueue []string
}

func (TestQMI) InitQueues(_ context.Context, _ tektonVersionedClient.Interface, _ pacVersionedClient.Interface) error {
	// TODO implement me
	panic("implement me")
}

func (TestQMI) RemoveRepository(_ *pacv1alpha1.Repository) {
}

func (t TestQMI) QueuedPipelineRuns(_ *pacv1alpha1.Repository) []string {
	return t.QueuedPrs
}

func (TestQMI) RunningPipelineRuns(_ *pacv1alpha1.Repository) []string {
	// TODO implement me
	panic("implement me")
}

func (t TestQMI) AddListToRunningQueue(_ *pacv1alpha1.Repository, _ []string) ([]string, error) {
	return t.RunningQueue, nil
}

func (TestQMI) AddToPendingQueue(_ *pacv1alpha1.Repository, _ []string) error {
	// TODO implement me
	panic("implement me")
}

func (t TestQMI) RemoveFromQueue(_, _ string) bool {
	return false
}

func (TestQMI) RemoveAndTakeItemFromQueue(_ *pacv1alpha1.Repository, _ *tektonv1.PipelineRun) string {
	// TODO implement me
	panic("implement me")
}

func (TestQMI) RequeueToPending(_ *pacv1alpha1.Repository, _ *tektonv1.PipelineRun) bool {
	return false
}

func (TestQMI) RequeueToPendingByKey(_, _ string) bool {
	return false
}
