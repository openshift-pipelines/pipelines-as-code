package resolve

import (
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/prompt"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestDetectWebhookSecret(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "detects webhook secret single quote",
			content: "secretName: '{{ git_auth_secret }}'",
			want:    true,
		},
		{
			name:    "detects webhook secret no quote",
			content: "secretName: {{ git_auth_secret }}",
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
		filenames          []string
		token              string
		params, fakeEnvs   map[string]string
		matchedSecretValue string
	}
	tests := []struct {
		name           string
		args           args
		wantOutputRe   string
		wantErr        bool
		wantSecretName string
		askStubs       func(*prompt.AskStubber)
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
			wantOutputRe: `.*git-credentials.*SHH_IAM_HIDDEN`,
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
			wantOutputRe: `.*git-credentials.*SOMUCHFUN`,
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
			wantOutputRe: `.*git-credentials.*TOKENARETHEBEST`,
		},
		{
			name: "provided a token via existing secret",
			args: args{
				filenames: []string{"testdata/pipelinerun.yaml"},
				params: map[string]string{
					"repo_url":   "https://forge/owner/repo",
					"repo_owner": "owner",
					"repo_name":  "name",
					"revision":   "https://forge/owner/12345",
				},
				matchedSecretValue: "EXISTINGSECRET",
			},
			wantSecretName: "existing-secret",
			wantOutputRe:   "^$",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			envRemove := env.PatchAll(t, tt.args.fakeEnvs)
			defer envRemove()

			as, teardown := prompt.InitAskStubber()
			defer teardown()
			if tt.askStubs != nil {
				tt.askStubs(as)
			}

			run := &params.Run{
				Info: info.Info{},
			}

			if tt.args.matchedSecretValue != "" {
				run.Info = info.Info{
					Kube: &info.KubeOpts{Namespace: "ns"},
				}
				tdata := testclient.Data{
					Namespaces: []*corev1.Namespace{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "ns",
							},
						},
					},
					Secret: []*corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      tt.wantSecretName,
								Namespace: "ns",
								Labels: map[string]string{
									keys.URLRepository: tt.args.params["repo_name"],
									keys.URLOrg:        tt.args.params["repo_owner"],
								},
							},
							Data: map[string][]byte{
								gitProviderTokenKey: []byte(base64.RawStdEncoding.EncodeToString([]byte(tt.args.matchedSecretValue))),
							},
						},
					},
				}
				stdata, _ := testclient.SeedTestData(t, ctx, tdata)
				run.Clients.Kube = stdata.Kube
			}

			got, secretName, err := makeGitAuthSecret(ctx, run, tt.args.filenames, tt.args.token, tt.args.params)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			if tt.wantSecretName != "" {
				assert.Equal(t, tt.wantSecretName, secretName)
			}
			assert.NilError(t, err)
			reg := regexp.MustCompile(tt.wantOutputRe)
			assert.Assert(t, reg.MatchString(got), "want: %s, got: %s", tt.wantOutputRe, got)
		})
	}
}
