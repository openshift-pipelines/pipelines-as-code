package policy

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func newRepoWithPolicy(policy *v1alpha1.Policy) *v1alpha1.Repository {
	return &v1alpha1.Repository{
		Spec: v1alpha1.RepositorySpec{
			Settings: &v1alpha1.Settings{
				Policy: policy,
			},
		},
	}
}

func TestPolicy_IsAllowed(t *testing.T) {
	type fields struct {
		repository *v1alpha1.Repository
		event      *info.Event
	}
	type args struct {
		tType info.TriggerType
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		replyAllowed bool
		want         bool
		wantErr      bool
		wantReason   string
	}{
		{
			name: "Test Policy.IsAllowed with no settings",
			fields: fields{
				repository: nil,
				event:      nil,
			},
			args: args{
				tType: info.TriggerTypePush,
			},
			want: false,
		},
		{
			name: "Test Policy.IsAllowed with unknown event type",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"pull_request"}}),
				event:      nil,
			},
			args: args{
				tType: "unknown",
			},
			want: false,
		},
		{
			name: "allowing member not in team for pull request",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"pull_request"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			replyAllowed: true,
			want:         true,
		},
		{
			name: "empty settings policy ignore",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{""}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			want: false,
		},
		{
			name: "disallowing member not in team for pull request",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"pull_request"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "allowing member in team for ok-to-test",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"ok-to-test"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeOkToTest,
			},
			replyAllowed: true,
			want:         false,
		},
		{
			name: "disallowing member not in team for ok-to-test",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"ok-to-test"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeOkToTest,
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "allowing member not in team for retest",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"ok-to-test"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeRetest,
			},
			want: false,
		},
		{
			name: "disallowing member not in team for retest",
			fields: fields{
				repository: newRepoWithPolicy(&v1alpha1.Policy{PullRequest: []string{"ok-to-test"}}),
				event:      info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeRetest,
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()

			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})

			vcx := &testprovider.TestProviderImp{PolicyDisallowing: !tt.replyAllowed}
			if tt.fields.event == nil {
				tt.fields.event = info.NewEvent()
			}
			p := &Policy{
				Repository:   tt.fields.repository,
				Event:        tt.fields.event,
				VCX:          vcx,
				Logger:       logger,
				EventEmitter: events.NewEventEmitter(stdata.Kube, logger),
			}
			got, reason := p.IsAllowed(ctx, tt.args.tType)
			if got != tt.want {
				t.Errorf("Policy.IsAllowed() = %v, want %v", got, tt.want)
			}
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}
