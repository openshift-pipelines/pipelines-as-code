package policy

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestPolicy_IsAllowed(t *testing.T) {
	type fields struct {
		Settings *v1alpha1.Settings
		Event    *info.Event
	}
	type args struct {
		tType info.TriggerType
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		replyAllowed bool
		want         Result
		wantErr      bool
	}{
		{
			name: "Test Policy.IsAllowed with no settings",
			fields: fields{
				Settings: nil,
				Event:    nil,
			},
			args: args{
				tType: info.TriggerTypePush,
			},
			want: ResultNotSet,
		},
		{
			name: "Test Policy.IsAllowed with unknown event type",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						PullRequest: []string{"pull_request"},
					},
				},
				Event: nil,
			},
			args: args{
				tType: "unknown",
			},
			want: ResultNotSet,
		},
		{
			name: "allowing member not in team for pull request",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						PullRequest: []string{"pull_request"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			replyAllowed: true,
			want:         ResultAllowed,
		},
		{
			name: "empty settings policy ignore",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						PullRequest: []string{},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			replyAllowed: true,
			want:         ResultNotSet,
		},
		{
			name: "disallowing member not in team for pull request",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						PullRequest: []string{"pull_request"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypePullRequest,
			},
			replyAllowed: false,
			want:         ResultDisallowed,
			wantErr:      true,
		},
		{
			name: "allowing member not in team for ok-to-test",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						OkToTest: []string{"ok-to-test"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeOkToTest,
			},
			replyAllowed: true,
			want:         ResultAllowed,
		},
		{
			name: "disallowing member not in team for ok-to-test",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						OkToTest: []string{"ok-to-test"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeOkToTest,
			},
			replyAllowed: false,
			want:         ResultDisallowed,
			wantErr:      true,
		},
		{
			name: "allowing member not in team for retest",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						OkToTest: []string{"ok-to-test"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeRetest,
			},
			replyAllowed: true,
			want:         ResultAllowed,
		},
		{
			name: "disallowing member not in team for retest",
			fields: fields{
				Settings: &v1alpha1.Settings{
					Policy: &v1alpha1.Policy{
						OkToTest: []string{"ok-to-test"},
					},
				},
				Event: info.NewEvent(),
			},
			args: args{
				tType: info.TriggerTypeRetest,
			},
			replyAllowed: false,
			want:         ResultDisallowed,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()

			vcx := &testprovider.TestProviderImp{PolicyDisallowing: !tt.replyAllowed}
			p := &Policy{
				Settings: tt.fields.Settings,
				Event:    tt.fields.Event,
				VCX:      vcx,
				Logger:   logger,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			got, err := p.IsAllowed(ctx, tt.args.tType)
			if (err != nil) != tt.wantErr {
				t.Errorf("Policy.IsAllowed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Policy.IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
