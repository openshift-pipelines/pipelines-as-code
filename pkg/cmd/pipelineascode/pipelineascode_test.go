package pipelineascode

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/consoleui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	kitesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestRunByProvider(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)

	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})

	tests := []struct {
		name                 string
		wantErr              bool
		event                *info.Event
		payloadFile          string
		consoleURLErroring   bool
		createStatusErroring bool
		tproviderimp         *testprovider.TestProviderImp
	}{
		{
			name:         "error on payload not here",
			payloadFile:  "/no/men/no/war",
			wantErr:      true,
			tproviderimp: &testprovider.TestProviderImp{},
		},
		{
			name:         "no payload passed",
			payloadFile:  "",
			wantErr:      true,
			tproviderimp: &testprovider.TestProviderImp{},
		},
		{
			name: "run pull_request user not allowed",
			event: &info.Event{
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
				BaseBranch:    "testbranch",
				DefaultBranch: "main",
				HeadBranch:    "main",
				Owner:         "owner",
				Repository:    "repo",
				SHA:           "sha",
				Sender:        "sender",
			},
			tproviderimp: &testprovider.TestProviderImp{AllowIT: false},
			payloadFile:  "testdata/pull_request.json",
		},
		{
			name: "no console URL set, just a warning",
			event: &info.Event{
				EventType:     "pull_request",
				TriggerTarget: "pull_request",
				BaseBranch:    "testbranch",
				DefaultBranch: "main",
				HeadBranch:    "main",
				Owner:         "owner",
				Repository:    "repo",
				SHA:           "sha",
				Sender:        "sender",
			},
			tproviderimp:       &testprovider.TestProviderImp{AllowIT: false},
			payloadFile:        "testdata/pull_request.json",
			consoleURLErroring: true,
		},
		{
			name:         "create status error",
			event:        &info.Event{},
			tproviderimp: &testprovider.TestProviderImp{AllowIT: false, CreateStatusErorring: true},
			payloadFile:  "testdata/pull_request.json",
			wantErr:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &params.Run{
				Clients: clients.Clients{
					Log:            fakelogger,
					PipelineAsCode: stdata.PipelineAsCode,
					ConsoleUI:      consoleui.FallBackConsole{},
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						WebhookType:   "github",
						PayloadFile:   test.payloadFile,
						ProviderToken: "TOKEN",
						ProviderURL:   "http://goninjago",
					},
				},
			}

			test.tproviderimp.Event = test.event
			kinteract := &kitesthelper.KinterfaceTest{
				ConsoleURL:         "http://console",
				ConsoleURLErorring: test.consoleURLErroring,
			}
			err := runWrap(ctx, cs, test.tproviderimp, kinteract)
			if test.wantErr {
				assert.Assert(t, err != nil, "We want an error here")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestGetPayloadFromFile(t *testing.T) {
	tests := []struct {
		name         string
		providertype string
		want         string
		wantErr      bool
	}{
		{
			name:         "Test Github",
			providertype: "github",
		},
		{
			name:         "Not recognized",
			providertype: "notrecognized",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &info.PacOpts{
				WebhookType: tt.providertype,
			}
			_, err := getGitProvider(opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPayloadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
