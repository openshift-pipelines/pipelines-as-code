package adapter

import (
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitlab"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/kubernetestint"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const fakePrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC6GorZBeri0eVERMZQDFh5E1RMPjFk9AevaWr27yJse6eiUlos
gY2L2vcZKLOrdvVR+TLLapIMFfg1E1qVr1iTHP3IiSCs1uW6NKDmxEQc9Uf/fG9c
i56tGmTVxLkC94AvlVFmgxtWfHdP3lF2O0EcfRyIi6EIbGkWDqWQVEQG2wIDAQAB
AoGAaKOd6FK0dB5Si6Uj4ERgxosAvfHGMh4n6BAc7YUd1ONeKR2myBl77eQLRaEm
DMXRP+sfDVL5lUQRED62ky1JXlDc0TmdLiO+2YVyXI5Tbej0Q6wGVC25/HedguUX
fw+MdKe8jsOOXVRLrJ2GfpKZ2CmOKGTm/hyrFa10TmeoTxkCQQDa4fvqZYD4vOwZ
CplONnVk+PyQETj+mAyUiBnHEeLpztMImNLVwZbrmMHnBtCNx5We10oCLW+Qndfw
Xi4LgliVAkEA2amSV+TZiUVQmm5j9yzon0rt1FK+cmVWfRS/JAUXyvl+Xh/J+7Gu
QzoEGJNAnzkUIZuwhTfNRWlzURWYA8BVrwJAZFQhfJd6PomaTwAktU0REm9ulTrP
vSNE4PBhoHX6ZOGAqfgi7AgIfYVPm+3rupE5a82TBtx8vvUa/fqtcGkW4QJAaL9t
WPUeJyx/XMJxQzuOe1JA4CQt2LmiBLHeRoRY7ephgQSFXKYmed3KqNT8jWOXp5DY
Q1QWaigUQdpFfNCrqwJBANLgWaJV722PhQXOCmR+INvZ7ksIhJVcq/x1l2BYOLw2
QsncVExbMiPa9Oclo5qLuTosS8qwHm1MJEytp3/SkB8=
-----END RSA PRIVATE KEY-----`

func Test_compareSecret(t *testing.T) {
	type args struct {
		incomingSecret string
		secretValue    string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "good/secret comparaison",
			args: args{
				incomingSecret: "foo",
				secretValue:    "foo",
			},
			want: true,
		},
		{
			name: "bad/secret comparaison",
			args: args{
				incomingSecret: "foo",
				secretValue:    "bar",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareSecret(tt.args.incomingSecret, tt.args.secretValue); got != tt.want {
				t.Errorf("compareSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_listener_detectIncoming(t *testing.T) {
	const goodURL = "https://matched/by/incoming"
	envRemove := env.PatchAll(t, map[string]string{"SYSTEM_NAMESPACE": "pipelinesascode"})
	defer envRemove()
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: os.Getenv("SYSTEM_NAMESPACE"),
		},
	}

	type args struct {
		data             testclient.Data
		method           string
		queryURL         string
		queryRepository  string
		queryPipelineRun string
		querySecret      string
		queryBranch      string
		secretResult     map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "good/incoming",
			want: true,
			args: args{
				secretResult: map[string]string{"good-secret": "verysecrete"},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-good",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"main"},
										Secret: v1alpha1.Secret{
											Name: "good-secret",
										},
									},
								},
								GitProvider: &v1alpha1.GitProvider{
									Type: "github",
								},
							},
						},
					},
				},
				method:           "GET",
				queryURL:         "/incoming",
				queryRepository:  "test-good",
				querySecret:      "verysecrete",
				queryPipelineRun: "pipelinerun1",
				queryBranch:      "main",
			},
		},
		{
			name: "bad/noincomingurl",
			args: args{
				queryURL: "/nowhere",
			},
			want: false,
		},
		{
			name: "bad/no repository in query",
			args: args{
				queryURL:         "/incoming",
				querySecret:      "verysecrete",
				queryPipelineRun: "pipelinerun1",
				queryBranch:      "main",
			},
			wantErr: true,
		},
		{
			name: "bad/no repository in query",
			args: args{
				queryURL:         "/incoming",
				querySecret:      "verysecrete",
				queryPipelineRun: "pipelinerun1",
				queryBranch:      "main",
				queryRepository:  "",
			},
			wantErr: true,
		},
		{
			name: "bad/no secret in query",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "",
				queryBranch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "bad/no pr in query",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "",
				querySecret:      "secret",
				queryBranch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "bad/no branch in query",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "",
			},
			wantErr: true,
		},
		{
			name: "bad/no matched repo",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "branch",
			},
			wantErr: true,
		},
		{
			name: "bad/no incomings",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "main",
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "repo",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"notmain"},
										Secret: v1alpha1.Secret{
											Name: "good-secret",
										},
									},
								},
								GitProvider: &v1alpha1.GitProvider{
									Type: "github",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bad/no matched branch in incoming",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "test-good",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "branch",
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-good",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								GitProvider: &v1alpha1.GitProvider{
									Type: "github",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bad/not git provider type provided",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "main",
				secretResult:     map[string]string{"secret": "secret"},
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "repo",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"main"},
										Secret: v1alpha1.Secret{
											Name: "secret",
										},
									},
								},
								GitProvider: &v1alpha1.GitProvider{},
							},
						},
					},
					Secret: []*corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Name:      pipelineascode.DefaultPipelinesAscodeSecretName,
							Namespace: testNamespace.Name,
						},
						Data: map[string][]byte{
							"github-application-id": []byte("274799"),
							"github-private-key":    []byte(fakePrivateKey),
						},
					}},
				},
			},
			wantErr: true,
		},
		{
			name: "bad/no matched secret",
			args: args{
				secretResult:     map[string]string{"secret": "verysecrete"},
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "main",
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "repo",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"main"},
										Secret: v1alpha1.Secret{
											Name: "secret",
										},
									},
								},
								GitProvider: &v1alpha1.GitProvider{
									Type: "github",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "bad/no matched secret",
			args: args{
				secretResult:     map[string]string{"secret": "verysecrete"},
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "main",
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "repo",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"main"},
										Secret: v1alpha1.Secret{
											Name: "secret",
										},
									},
								},
								GitProvider: &v1alpha1.GitProvider{
									Type: "github",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no git provider",
			args: args{
				queryURL:         "/incoming",
				queryRepository:  "repo",
				queryPipelineRun: "pr",
				querySecret:      "secret",
				queryBranch:      "main",
				data: testclient.Data{
					Repositories: []*v1alpha1.Repository{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "repo",
							},
							Spec: v1alpha1.RepositorySpec{
								URL: goodURL,
								Incomings: &[]v1alpha1.Incoming{
									{
										Targets: []string{"main"},
										Secret: v1alpha1.Secret{
											Name: "secret",
										},
									},
								},
							},
						},
					},
					Secret: []*corev1.Secret{{
						ObjectMeta: metav1.ObjectMeta{
							Name:      pipelineascode.DefaultPipelinesAscodeSecretName,
							Namespace: testNamespace.Name,
						},
						Data: map[string][]byte{
							"github-application-id": []byte("274799"),
							"github-private-key":    []byte(fakePrivateKey),
						},
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			cs, _ := testclient.SeedTestData(t, ctx, tt.args.data)
			client := &params.Run{
				Clients: clients.Clients{
					PipelineAsCode: cs.PipelineAsCode,
					Kube:           cs.Kube,
				},
				Info: info.Info{},
			}
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			kint := &kubernetestint.KinterfaceTest{
				GetSecretResult: tt.args.secretResult,
			}

			l := &listener{
				run:    client,
				logger: logger,
				kint:   kint,
				event:  info.NewEvent(),
			}
			// make a new request
			req := httptest.NewRequest(tt.args.method,
				fmt.Sprintf("http://localhost%s?repository=%s&secret=%s&pipelinerun=%s&branch=%s", tt.args.queryURL,
					tt.args.queryRepository, tt.args.querySecret, tt.args.queryPipelineRun, tt.args.queryBranch),
				strings.NewReader(""))
			got, _, err := l.detectIncoming(ctx, req, []byte(""))
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.Equal(t, got, tt.want)
			assert.Equal(t, l.event.TargetPipelineRun, tt.args.queryPipelineRun)
		})
	}
}

func Test_listener_processIncoming(t *testing.T) {
	tests := []struct {
		name       string
		want       provider.Interface
		wantErr    bool
		targetRepo *v1alpha1.Repository
		wantOrg    string
		wantRepo   string
	}{
		{
			name:     "process/github",
			want:     github.New(),
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{
						Type: "github",
					},
				},
			},
		},
		{
			name:     "process/gitlab",
			want:     &gitlab.Provider{},
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{
						Type: "gitlab",
					},
				},
			},
		},
		{
			name:     "process/bitbucketcloud",
			want:     &bitbucketcloud.Provider{},
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{
						Type: "bitbucket-cloud",
					},
				},
			},
		},
		{
			name:     "process/bitbucketserver",
			want:     &bitbucketserver.Provider{},
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{
						Type: "bitbucket-server",
					},
				},
			},
		},
		{
			name:    "error/unknown provider",
			wantErr: true,
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{
						Type: "unknown",
					},
				},
			},
		},
		{
			name:    "error/bad url",
			wantErr: true,
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "hellomoto",
				},
			},
		},
		{
			name:    "error/not enough path in url",
			wantErr: true,
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL: "https://forge?owner=repo",
				},
			},
		},
		{
			name:     "No GitProvider is provided",
			want:     github.New(),
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL:         "https://forge/owner/repo",
					GitProvider: nil,
				},
			},
		},
		{
			name:     "No GitProvider type is provided",
			want:     github.New(),
			wantOrg:  "owner",
			wantRepo: "repo",
			targetRepo: &v1alpha1.Repository{
				Spec: v1alpha1.RepositorySpec{
					URL:         "https://forge/owner/repo",
					GitProvider: &v1alpha1.GitProvider{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &params.Run{
				Info: info.Info{},
			}
			kint := &kubernetestint.KinterfaceTest{}
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			l := &listener{
				run: client, kint: kint, logger: logger, event: info.NewEvent(),
			}
			pintf, _, err := l.processIncoming(tt.targetRepo)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.Assert(t, reflect.TypeOf(pintf).Elem() == reflect.TypeOf(tt.want).Elem())
			assert.Assert(t, l.event.Organization == tt.wantOrg)
			assert.Assert(t, l.event.Repository == tt.wantRepo)
		})
	}
}
