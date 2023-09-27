package generate

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

func TestGenerateTemplate(t *testing.T) {
	tests := []struct {
		name                    string
		wantErrStr              string
		askStubs                func(*prompt.AskStubber)
		runInfo                 info.Info
		gitinfo                 git.Info
		repo                    apipac.Repository
		wantStdout              string
		event                   info.Event
		wantURL                 string
		checkGeneratedFile      string
		checkRegInGeneratedFile []*regexp.Regexp
		addExtraFilesInRepo     map[string]string
		regenerateTemplate      bool
	}{
		{
			name: "pull request default",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOneDefault() // pull_request
				as.StubOne("")      // default as main
				as.StubOne(true)    // pipelinerun generation
			},
			checkGeneratedFile: ".tekton/pull-request.yaml",
			checkRegInGeneratedFile: []*regexp.Regexp{
				regexp.MustCompile("name: moto-pull-request"),
				regexp.MustCompile(".*on-event.*pull_request"),
			},
			gitinfo: git.Info{
				URL: "https://hello/moto",
			},
			regenerateTemplate: true,
		},
		{
			name: "pull request already exist don't overwrite",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOneDefault() // pull_request
				as.StubOne("")      // default as main
				as.StubOne(false)   // overwrite
			},
			addExtraFilesInRepo: map[string]string{
				".tekton/pull-request.yaml": "hello moto",
			},
			checkGeneratedFile: ".tekton/pull-request.yaml",
			checkRegInGeneratedFile: []*regexp.Regexp{
				regexp.MustCompile("hello moto"),
			},
			gitinfo: git.Info{
				URL: "https://hello/moto",
			},
			regenerateTemplate: true,
		},
		{
			name: "pull request golang",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOneDefault() // pull_request
				as.StubOne("")      // default as main
			},
			addExtraFilesInRepo: map[string]string{
				"go.mod": "random string",
			},
			checkGeneratedFile: ".tekton/pull-request.yaml",
			checkRegInGeneratedFile: []*regexp.Regexp{
				regexp.MustCompile("name: golang-pull-request"),
				regexp.MustCompile(".*on-event.*pull_request"),
				regexp.MustCompile(".*test our Golang project"),
				regexp.MustCompile("- name: golangci-lint"),
			},
			gitinfo: git.Info{
				URL: "https://hello/golang",
			},
			regenerateTemplate: true,
		},
		{
			name: "pull request python",
			askStubs: func(as *prompt.AskStubber) {
				// I can't see to make the stubbing work for push :\
				as.StubOneDefault() // pull_request
				as.StubOne("")      // default as main
			},
			addExtraFilesInRepo: map[string]string{
				"setup.py": "random string",
			},
			checkGeneratedFile: ".tekton/pull-request.yaml",
			checkRegInGeneratedFile: []*regexp.Regexp{
				regexp.MustCompile("name: pythonrulez-pull-request"),
				regexp.MustCompile(".*on-event.*pull_request"),
				regexp.MustCompile(".*test our Python project"),
				regexp.MustCompile("- name: pylint"),
			},
			gitinfo: git.Info{
				URL: "https://hello/pythonrulez",
			},
			regenerateTemplate: true,
		},
		{
			name: "pull request already exist don't regenerate sample template",
			askStubs: func(as *prompt.AskStubber) {
				as.StubOneDefault() // pull_request
				as.StubOne("")      // default as main
			},
			addExtraFilesInRepo: map[string]string{
				".tekton/pull-request.yaml": "hello moto",
			},
			checkGeneratedFile: ".tekton/pull-request.yaml",
			checkRegInGeneratedFile: []*regexp.Regexp{
				regexp.MustCompile("hello moto"),
			},
			gitinfo: git.Info{
				URL: "https://hello/moto",
			},
			regenerateTemplate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}
			io, _, _, _ := cli.IOTest()

			newdir := fs.NewDir(t, "TestGenerate")
			defer newdir.Remove()
			tt.gitinfo.TopLevelPath = newdir.Path()

			for key, value := range tt.addExtraFilesInRepo {
				// make sure the dir is created
				err := os.MkdirAll(filepath.Dir(newdir.Join(key)), os.ModePerm)
				assert.NilError(t, err, "failed to create dir: %s", filepath.Dir(newdir.Join(key)))

				err = os.WriteFile(newdir.Join(key), []byte(value), 0o600)
				assert.NilError(t, err, "failed to create file", key)
			}

			err := Generate(&Opts{
				Event:     &tt.event,
				GitInfo:   &tt.gitinfo,
				IOStreams: io,
				CLIOpts:   &cli.PacCliOpts{},
			}, tt.regenerateTemplate)
			assert.NilError(t, err)

			// check if file has been generated
			if tt.checkGeneratedFile != "" {
				// check if file exists
				_, err := os.Stat(newdir.Join(tt.checkGeneratedFile))
				assert.Assert(t, !os.IsNotExist(err))
			}
			if tt.checkRegInGeneratedFile != nil {
				// check if file contains the expected strings
				b, err := os.ReadFile(newdir.Join(tt.checkGeneratedFile))
				assert.NilError(t, err)
				for _, s := range tt.checkRegInGeneratedFile {
					// check if regexp matches
					assert.Assert(t, s.Match(b), "cannot match regexp %s in file: %s", s, string(b))
				}
			}
		})
	}
}
