package customparams

import (
	"testing"

	"github.com/google/go-github/v50/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestProcessTemplates(t *testing.T) {
	ns := "there"
	tests := []struct {
		name               string
		event              *info.Event
		expected           map[string]string
		repository         *v1alpha1.Repository
		secretData         map[string]string
		expectedLogSnippet string
		expectedError      bool
	}{
		{
			name:     "params/basic",
			expected: map[string]string{"params": "batman"},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:  "params",
							Value: "batman",
						},
					},
				},
			},
		},
		{
			name:     "params/from secret",
			expected: map[string]string{"params": "gone", "target_namespace": ns},
			secretData: map[string]string{
				"name": "gone",
			},
			repository: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
				},
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name: "params",
							SecretRef: &v1alpha1.Secret{
								Name: "name",
								Key:  "key",
							},
						},
					},
				},
			},
		},
		{
			name:     "params/use last params when two values of the same name",
			expected: map[string]string{"params": "robin"},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:  "params",
							Value: "batman",
						},
						{
							Name:  "params",
							Value: "robin",
						},
					},
				},
			},
		},
		{
			name:     "params/fallback to stdparams",
			expected: map[string]string{"event_type": "pull_request"},
			event:    &info.Event{EventType: "pull_request"},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{},
				},
			},
		},
		{
			name:               "params/skip with no name",
			expectedLogSnippet: "no name has been set in params[0] of repo",
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Value: "batman",
						},
					},
				},
			},
		},
		{
			name:               "params/pick value when value and secret set",
			expected:           map[string]string{"params": "batman"},
			expectedLogSnippet: "repo repo, param name params has a value and secretref, picking value",
			repository: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repo",
				},
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:  "params",
							Value: "batman",
							SecretRef: &v1alpha1.Secret{
								Name: "name",
								Key:  "key",
							},
						},
					},
				},
			},
		},
		{
			name:     "params/filter",
			expected: map[string]string{"event_type": "pull_request", "params": "batman"},
			event:    &info.Event{EventType: "pull_request"},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `pac.event_type == "pull_request"`,
						},
					},
				},
			},
		},
		{
			name:     "params/filter on body",
			expected: map[string]string{"params": "batman", "event_type": "pull_request"},
			event:    &info.Event{EventType: "pull_request", Event: github.PullRequestEvent{Number: github.Int(42)}},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `body.number == 42`,
						},
					},
				},
			},
		},
		{
			name:          "params/filter on body with bad filter",
			event:         &info.Event{EventType: "pull_request", Event: github.PullRequestEvent{Number: github.Int(42)}},
			expectedError: true,
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `body.BADDDADA == 42`,
						},
					},
				},
			},
		},
		{
			name:          "params/bad filter skipped",
			event:         &info.Event{EventType: "push"},
			expectedError: true,
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `BADDADADDADA`,
						},
					},
				},
			},
		},
		{
			name:          "params/bad payload skipped",
			event:         &info.Event{EventType: "push", Event: "BADADDADA"},
			expectedError: true,
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `body.number == 42`,
						},
					},
				},
			},
		},
		{
			name:               "params/no filter match",
			expectedLogSnippet: "skipping params name params, filter condition is false",
			event:              &info.Event{EventType: "push"},
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman",
							Filter: `pac.event_type == "pull_request"`,
						},
					},
				},
			},
		},
		{
			name:               "params/two filters same name, match first",
			expected:           map[string]string{"params": "batman1", "event_type": "pull_request"},
			event:              &info.Event{EventType: "pull_request"},
			expectedLogSnippet: "skipping params name params, filter has already been matched previously",
			repository: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					Params: &[]v1alpha1.Params{
						{
							Name:   "params",
							Value:  "batman1",
							Filter: `pac.event_type == "pull_request"`,
						},
						{
							Name:   "params",
							Value:  "batman2",
							Filter: `pac.event_type == "pull_request"`,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, log := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			repo := tt.repository
			if repo == nil {
				repo = &v1alpha1.Repository{}
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			run := &params.Run{Clients: clients.Clients{}}
			if tt.event == nil {
				tt.event = &info.Event{}
			}
			p := NewCustomParams(tt.event, repo, run, &kitesthelper.KinterfaceTest{GetSecretResult: tt.secretData}, nil)
			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
			p.eventEmitter = events.NewEventEmitter(stdata.Kube, logger)
			ret, err := p.GetParams(ctx)
			if tt.expectedError {
				assert.Assert(t, err != nil, "expected error but got nil")
			} else {
				assert.NilError(t, err)
			}
			if len(tt.expected) > 0 {
				assert.DeepEqual(t, tt.expected, ret)
			}
			if tt.expectedLogSnippet != "" {
				logmsg := log.FilterMessageSnippet(tt.expectedLogSnippet).TakeAll()
				assert.Assert(t, len(logmsg) > 0, "log message filtered %s expected %s all logs: %+v", logmsg,
					tt.expectedLogSnippet, log.TakeAll())
			}
		})
	}
}
