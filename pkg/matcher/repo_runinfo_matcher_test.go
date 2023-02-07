package matcher

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	mainBranch      = "mainBranch"
	targetNamespace = "targetNamespace"
	targetURL       = "https//nowhere.togo"
)

func Test_getRepoByCR(t *testing.T) {
	type args struct {
		data     testclient.Data
		runevent info.Event
	}
	tests := []struct {
		name         string
		args         args
		wantTargetNS string
		wantErr      bool
	}{
		{
			name: "test-match",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-match-url-slash-at-the-end",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              "https//nowhere.togo/",
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-nomatch-url",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              "http://nottarget.url",
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: "",
			wantErr:      false,
		},
		{
			name: "straightforward-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{
					URL: targetURL, BaseBranch: "refs/heads/mainBranch",
					EventType: "pull_request",
				},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "glob-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						testnewrepo.NewRepo(
							testnewrepo.RepoTestcreationOpts{
								Name:             "test-good",
								URL:              targetURL,
								InstallNamespace: targetNamespace,
							},
						),
					},
				},
				runevent: info.Event{
					URL:        targetURL,
					BaseBranch: "refs/tags/1.0",
					EventType:  "pull_request",
				},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			cs, _ := testclient.SeedTestData(t, ctx, tt.args.data)
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			client := &params.Run{
				Clients: clients.Clients{PipelineAsCode: cs.PipelineAsCode, Log: logger},
				Info:    info.Info{},
			}
			got, err := MatchEventURLRepo(ctx, client, &tt.args.runevent, "")

			if err == nil && tt.wantErr {
				assert.NilError(t, err, "GetRepoByCR() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTargetNS == "" && got != nil {
				t.Errorf("GetRepoByCR() got = '%v', want '%v'", got.GetNamespace(), tt.wantTargetNS)
			}
			if tt.wantTargetNS != "" && got == nil {
				t.Errorf("GetRepoByCR() want nil got '%v'", tt.wantTargetNS)
			}

			if tt.wantTargetNS != "" && tt.wantTargetNS != got.GetNamespace() {
				t.Errorf("GetRepoByCR() got = '%v', want '%v'", got.GetNamespace(), tt.wantTargetNS)
			}
		})
	}
}
