//go:build e2e
// +build e2e

package test

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRepoValidation(t *testing.T) {
	ctx := context.TODO()
	run := params.New()
	assert.NilError(t, run.Clients.NewClients(ctx, &run.Info))
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-ns")
	assert.NilError(t, pacrepo.CreateNS(ctx, targetNS, run))

	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{
			name:        "not http or https",
			url:         "foobar",
			expectedErr: "URL scheme must be http or https",
		},
		{
			name:        "invalid URL",
			url:         "http://   ",
			expectedErr: "invalid URL format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNS,
				},
				Spec: v1alpha1.RepositorySpec{
					URL: tt.url,
				},
			}
			err := pacrepo.CreateRepo(ctx, targetNS, run, repository)
			assert.ErrorContains(t, err, tt.expectedErr)
		})
	}
}
