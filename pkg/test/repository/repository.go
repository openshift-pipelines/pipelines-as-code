package repository

import (
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck/v1beta1"
)

type RepoTestcreationOpts struct {
	Name             string
	URL              string
	InstallNamespace string
	SecretName       string
	ProviderURL      string
	CreateTime       metav1.Time
	RepoStatus       []v1alpha1.RepositoryRunStatus
}

func NewRepo(opts RepoTestcreationOpts) *v1alpha1.Repository {
	cw := clockwork.NewFakeClock()

	if opts.RepoStatus == nil {
		opts.RepoStatus = []v1alpha1.RepositoryRunStatus{
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun5",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-56 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-55 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun4",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-46 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-45 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun3",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-36 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-35 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun2",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-26 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-25 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun1",
				StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
				CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
			},
		}
	}

	repo := &v1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:              opts.Name,
			Namespace:         opts.InstallNamespace,
			CreationTimestamp: opts.CreateTime,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: opts.URL,
			GitProvider: &v1alpha1.GitProvider{
				Secret: &v1alpha1.GitProviderSecret{},
			},
		},
		Status: opts.RepoStatus,
	}
	if opts.SecretName != "" {
		repo.Spec.GitProvider.Secret = &v1alpha1.GitProviderSecret{
			Name: opts.SecretName,
		}
	}
	if opts.ProviderURL != "" {
		repo.Spec.GitProvider.URL = opts.ProviderURL
	}
	return repo
}
