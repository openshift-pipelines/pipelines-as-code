package repository

import (
	"context"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNSRepo(ctx context.Context, targetNS string, cs *cli.Clients,
	repository *pacv1alpha1.Repository) error {
	nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: targetNS}}
	_, err := cs.Kube.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	repo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Create(ctx, repository, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	cs.Log.Infof("Repository created in %s", repo.GetNamespace())
	return nil
}
