package resolve

import (
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

func TestDetectWebhookSecret(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "detects webhook secret",
			content: basicAuthSecretString,
			want:    true,
		},

		{
			name:    "not webhook secret detected",
			content: "foobar",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile := fs.NewFile(t, t.Name(), fs.WithContent(tt.content))
			defer tmpfile.Remove()
			filenames := []string{tmpfile.Path()}
			if got := detectWebhookSecret(filenames); got != tt.want {
				t.Errorf("detectWebhookSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeGitAuthSecret(t *testing.T) {
	type args struct {
		filenames        []string
		token            string
		params, fakeEnvs map[string]string
	}
	tests := []struct {
		name     string
		args     args
		want     string
		wantErr  bool
		askStubs func(*prompt.AskStubber)
	}{
		{
			name: "ask for provider token",
			args: args{
				filenames: []string{"testdata/pipelinerun.yaml"},
				params: map[string]string{
					"repo_url": "https://forge/owner/repo",
					"revision": "https://forge/owner/12345",
				},
			},
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne(true)
				as.StubOne("SHH_IAM_HIDDEN")
			},
			want: `.*git-credentials.*SHH_IAM_HIDDEN`,
		},
		{
			name: "do not care about token stuff",
			args: args{
				filenames: []string{"testdata/pipelinerun.yaml"},
				params: map[string]string{
					"repo_url": "https://forge/owner/repo",
					"revision": "https://forge/owner/12345",
				},
			},
			askStubs: func(as *prompt.AskStubber) {
				as.StubOne("n")
			},
			wantErr: false,
		},
		{
			name: "provided a token on flag",
			args: args{
				filenames: []string{"testdata/pipelinerun.yaml"},
				params: map[string]string{
					"repo_url": "https://forge/owner/repo",
					"revision": "https://forge/owner/12345",
				},
				token: "SOMUCHFUN",
			},
			want: `.*git-credentials.*SOMUCHFUN`,
		},
		{
			name: "provided a token via env",
			args: args{
				filenames: []string{"testdata/pipelinerun.yaml"},
				params: map[string]string{
					"repo_url": "https://forge/owner/repo",
					"revision": "https://forge/owner/12345",
				},
				fakeEnvs: map[string]string{
					"PAC_PROVIDER_TOKEN": "TOKENARETHEBEST",
				},
			},
			want: `.*git-credentials.*TOKENARETHEBEST`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRemove := env.PatchAll(t, tt.args.fakeEnvs)
			defer envRemove()

			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}

			got, _, err := makeGitAuthSecret(tt.args.filenames, tt.args.token, tt.args.params)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			reg := regexp.MustCompile(tt.want)
			assert.Assert(t, reg.MatchString(got), "want: %s, got: %s", tt.want, got)
		})
	}
}
