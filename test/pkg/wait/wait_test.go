package wait

import (
	"testing"
	"time"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	paramsclients "github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func makeRepositoryRunStatus(sha *string, reason string) pacv1alpha1.RepositoryRunStatus {
	status := pacv1alpha1.RepositoryRunStatus{
		SHA: sha,
	}
	if reason != "" {
		status.Status = duckv1.Status{
			Conditions: []knativeapis.Condition{
				{Reason: reason},
			},
		}
	}
	return status
}

func strPtr(v string) *string {
	return &v
}

func TestUntilRepositoryHasStatusReason(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []pacv1alpha1.RepositoryRunStatus
		targetSHA string
		reason    string
		wantErr   bool
	}{
		{
			name: "match by target sha and reason",
			statuses: []pacv1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(strPtr("sha-1"), "Succeeded"),
				makeRepositoryRunStatus(strPtr("sha-2"), "Cancelled"),
			},
			targetSHA: "sha-2",
			reason:    "Cancelled",
		},
		{
			name: "wrong reason for matching sha",
			statuses: []pacv1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(strPtr("sha-2"), "Succeeded"),
			},
			targetSHA: "sha-2",
			reason:    "Cancelled",
			wantErr:   true,
		},
		{
			name: "reason exists on a different sha only",
			statuses: []pacv1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(strPtr("sha-1"), "Cancelled"),
			},
			targetSHA: "sha-2",
			reason:    "Cancelled",
			wantErr:   true,
		},
		{
			name: "match without target sha filter",
			statuses: []pacv1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(strPtr("sha-1"), "Cancelled"),
			},
			reason: "Cancelled",
		},
		{
			name: "status without conditions",
			statuses: []pacv1alpha1.RepositoryRunStatus{
				makeRepositoryRunStatus(strPtr("sha-2"), ""),
			},
			targetSHA: "sha-2",
			reason:    "Cancelled",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			repositoryNS := "test-repository-ns"
			repositoryName := "test-repository"
			seeded, _ := testclient.SeedTestData(t, ctx, testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{Name: repositoryNS},
					},
				},
				Repositories: []*pacv1alpha1.Repository{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      repositoryName,
							Namespace: repositoryNS,
						},
						Spec: pacv1alpha1.RepositorySpec{
							URL: "https://ghe.pipelinesascode.com/org/repo",
						},
						Status: tt.statuses,
					},
				},
			})

			clients := paramsclients.Clients{
				PipelineAsCode: seeded.PipelineAsCode,
				Log:            zap.NewNop().Sugar(),
			}
			opts := Opts{
				RepoName:    repositoryName,
				Namespace:   repositoryNS,
				TargetSHA:   tt.targetSHA,
				PollTimeout: 10 * time.Millisecond,
			}

			repository, err := UntilRepositoryHasStatusReason(ctx, clients, opts, tt.reason)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				assert.ErrorContains(t, err, "timed out")
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, repository != nil)
			assert.Equal(t, repository.GetName(), repositoryName)
		})
	}
}
