package repository

import (
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck/v1beta1"
)

func NewRepo(name, url, branch, installNamespace, namespace, eventtype, secretname, vcsurl string) *v1alpha1.Repository {
	cw := clockwork.NewFakeClock()
	repo := &v1alpha1.Repository{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: installNamespace,
		},
		Spec: v1alpha1.RepositorySpec{
			Namespace: namespace,
			URL:       url,
			Branch:    branch,
			EventType: eventtype,
		},
		Status: []v1alpha1.RepositoryRunStatus{
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun5",
				StartTime:       &v1.Time{Time: cw.Now().Add(-56 * time.Minute)},
				CompletionTime:  &v1.Time{Time: cw.Now().Add(-55 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun4",
				StartTime:       &v1.Time{Time: cw.Now().Add(-46 * time.Minute)},
				CompletionTime:  &v1.Time{Time: cw.Now().Add(-45 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun3",
				StartTime:       &v1.Time{Time: cw.Now().Add(-36 * time.Minute)},
				CompletionTime:  &v1.Time{Time: cw.Now().Add(-35 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun2",
				StartTime:       &v1.Time{Time: cw.Now().Add(-26 * time.Minute)},
				CompletionTime:  &v1.Time{Time: cw.Now().Add(-25 * time.Minute)},
			},
			{
				Status:          v1beta1.Status{},
				PipelineRunName: "pipelinerun1",
				StartTime:       &v1.Time{Time: cw.Now().Add(-16 * time.Minute)},
				CompletionTime:  &v1.Time{Time: cw.Now().Add(-15 * time.Minute)},
			},
		},
	}
	if secretname != "" {
		repo.Spec.WebvcsSecret = &v1alpha1.WebvcsSecretSpec{
			Name: secretname,
		}
	}
	if vcsurl != "" {
		repo.Spec.WebvcsAPIURL = vcsurl
	}
	return repo
}
