package pipelineascode

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func Test_getRepoByCR(t *testing.T) {
	type args struct {
		data    testclient.Data
		runinfo *webvcs.RunInfo
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
						newRepo("test-good", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: targetNamespace,
			wantErr:      false,
		},
		{
			name: "test-nomatch-event-type",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						newRepo("test-good", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{URL: targetURL, BaseBranch: mainBranch, EventType: "push"},
			},
			wantTargetNS: "",
			wantErr:      false,
		},
		{
			name: "test-nomatch-base-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						newRepo("test-good", targetURL, mainBranch, targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{URL: targetURL, BaseBranch: "anotherBaseBranch", EventType: "pull_request"},
			},
			wantTargetNS: "",
			wantErr:      false,
		},
		{
			name: "test-nomatch-url",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						newRepo("test-good", "http://nottarget.url", mainBranch,
							targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{URL: targetURL, BaseBranch: mainBranch, EventType: "pull_request"},
			},
			wantTargetNS: "",
			wantErr:      false,
		},
		{
			name: "straightforward-branch",
			args: args{
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						newRepo("test-good", targetURL, mainBranch,
							targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{
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
						newRepo("test-good", targetURL, "refs/tags/*",
							targetNamespace, targetNamespace, "pull_request"),
					},
				},
				runinfo: &webvcs.RunInfo{
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
			client := &cli.Clients{PipelineAsCode: cs.PipelineAsCode}
			got, err := getRepoByCR(ctx, client, tt.args.runinfo)
			if tt.wantErr {
				assert.NilError(t, err, "getRepoByCR() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTargetNS == "" && got.Spec.Namespace != "" {
				t.Errorf("getRepoByCR() got = '%v', want '%v'", got.Spec.Namespace, tt.wantTargetNS)
			}
			if tt.wantTargetNS != "" && tt.wantTargetNS != got.Spec.Namespace {
				t.Errorf("getRepoByCR() got = '%v', want '%v'", got.Spec.Namespace, tt.wantTargetNS)
			}
		})
	}
}
