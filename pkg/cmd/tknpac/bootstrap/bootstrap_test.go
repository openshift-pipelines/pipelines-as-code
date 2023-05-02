package bootstrap

import (
	"bytes"
	"io"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func newIOStream() (*cli.IOStreams, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.IOStreams{
		In:     io.NopCloser(in),
		Out:    out,
		ErrOut: errOut,
	}, out
}

func TestInstall(t *testing.T) {
	ctx, _ := rtesting.SetupFakeContext(t)
	cs, _ := testclient.SeedTestData(t, ctx, testclient.Data{})
	logger, _ := logger.GetLogger()

	run := &params.Run{
		Clients: clients.Clients{
			PipelineAsCode: cs.PipelineAsCode,
			Log:            logger,
			Kube:           cs.Kube,
		},
		Info: info.Info{},
	}
	io, out := newIOStream()
	opts := &bootstrapOpts{ioStreams: io}
	err := install(ctx, run, opts)
	// get an error because i need to figure out how to fake dynamic client
	assert.Assert(t, err != nil)
	assert.Equal(t, "=> Checking if Pipelines as Code is installed.\n", out.String())
}
