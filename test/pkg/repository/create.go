package repository

import (
	"context"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNS(ctx context.Context, targetNS string, cs *params.Run) error {
	nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: targetNS}}
	_, err := cs.Clients.Kube.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	cs.Clients.Log.Infof("Namespace %s created", targetNS)
	return err
}

func CreateRepo(ctx context.Context, targetNS string, cs *params.Run,
	repository *pacv1alpha1.Repository) error {
	repo, err := cs.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Create(ctx, repository, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	cs.Clients.Log.Infof("Repository created in %s", repo.GetNamespace())
	return nil
}
