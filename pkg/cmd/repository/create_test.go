package repository

import (
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func Test_create(t *testing.T) {
	tests := []struct {
		name            string
		wantErr         bool
		targetNamespace string
		subsMatch       string
	}{
		{
			name:            "test",
			targetNamespace: "ns",
			subsMatch:       "has been created",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nd := fs.NewDir(t, "TestGetGitInfo")
			defer nd.Remove()
			gitDir := nd.Path()
			_, _ = git.RunGit(gitDir, "init")
			_, _ = git.RunGit(gitDir, "remote", "add", "origin", "https://url/owner/repo")

			ctx, _ := rtesting.SetupFakeContext(t)
			tdata := testclient.Data{}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &cli.Clients{
				PipelineAsCode: stdata.PipelineAsCode,
			}
			io, out := newIOStream()
			opts := CreateOptions{
				AssumeYes: true,
				Clients:   cs,
				IOStreams: io,
				CurrentNS: tt.targetNamespace,
			}

			err := create(ctx, gitDir, opts)
			assert.NilError(t, err)
			assert.Assert(t, strings.Contains(out.String(), tt.subsMatch))
		})
	}
}
