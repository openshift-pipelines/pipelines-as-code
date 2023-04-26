package pipelineascode

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

// script kiddies, don't get too excited, this has been randomly generated with random words
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

var (
	testInstallationID = int64(1234567)
	tempToken          = "123abcdfrf"
)

func TestPacRun_checkNeedUpdate(t *testing.T) {
	tests := []struct {
		name                 string
		tmpl                 string
		upgradeMessageSubstr string
		needupdate           bool
	}{
		{
			name:                 "old secrets",
			tmpl:                 `secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"`,
			upgradeMessageSubstr: "old basic auth secret name",
			needupdate:           true,
		},
		{
			name:       "no need",
			tmpl:       ` secretName: "foo-bar-foo"`,
			needupdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPacs(nil, nil, &params.Run{Clients: clients.Clients{}}, nil, nil)
			got, needupdate := p.checkNeedUpdate(tt.tmpl)
			if tt.upgradeMessageSubstr != "" {
				assert.Assert(t, strings.Contains(got, tt.upgradeMessageSubstr))
			}
			assert.Assert(t, needupdate == tt.needupdate)
		})
	}
}

func TestChangeSecret(t *testing.T) {
	prs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "{{git_auth_secret}}",
			},
		},
	}
	err := changeSecret(prs)
	assert.NilError(t, err)
	assert.Assert(t, strings.HasPrefix(prs[0].GetName(), "pac-gitauth"), prs[0].GetName(), "has no pac-gitauth prefix")
	assert.Assert(t, prs[0].GetAnnotations()[apipac.GitAuthSecret] != "")
}

func TestFilterRunningPipelineRunOnTargetTest(t *testing.T) {
	testPipeline := "test"
	prs := []*tektonv1.PipelineRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pipelinerun-" + testPipeline,
				Annotations: map[string]string{
					apipac.OriginalPRName: testPipeline,
				},
			},
		},
	}
	ret := filterRunningPipelineRunOnTargetTest("", prs)
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Equal(t, prs[0].GetName(), ret[0].GetName())
	prs = []*tektonv1.PipelineRun{}
	ret = filterRunningPipelineRunOnTargetTest(testPipeline, prs)
	assert.Assert(t, ret == nil)
}

func TestScopeTokenToListOfRipos(t *testing.T) {
	var (
		repoFromWhichEventComes = "https://org.com/owner/repo"
		privateRepo             = "https://org.com/owner2/repo2"
	)
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinesascode",
		},
	}

	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelines-as-code-secret",
			Namespace: testNamespace.Name,
		},
		Data: map[string][]byte{
			"github-application-id": []byte("12345"),
			"github-private-key":    []byte(fakePrivateKey),
		},
	}
	repoData := &v1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "publicrepo",
			Namespace: testNamespace.Name,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: repoFromWhichEventComes,
			Settings: &v1alpha1.Settings{
				GithubAppTokenScopeRepos: []string{"owner2/repo2"},
			},
		},
	}
	repoData1 := &v1alpha1.Repository{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "privaterepo",
			Namespace: testNamespace.Name,
		},
		Spec: v1alpha1.RepositorySpec{
			URL: privateRepo,
		},
	}
	tests := []struct {
		name                  string
		tData                 testclient.Data
		envs                  map[string]string
		repository            *v1alpha1.Repository
		repoListsByGlobalConf string
		wantError             string
		wantToken             string
		repositoryID          []int64
	}{
		{
			name: "repos are listed under global configuration",
			tData: testclient.Data{
				Namespaces: []*corev1.Namespace{testNamespace},
				Secret:     []*corev1.Secret{validSecret},
			},
			repository: &v1alpha1.Repository{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "publicrepo",
					Namespace: testNamespace.Name,
				},
				Spec: v1alpha1.RepositorySpec{
					URL: repoFromWhichEventComes,
				},
			},
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace.Name,
			},
			repoListsByGlobalConf: "owner1/repo1",
			wantError:             "",
			wantToken:             "123abcdfrf",
			repositoryID:          []int64{789, 10112},
		},
		{
			name: "repos are listed under repo level configuration",
			tData: testclient.Data{
				Namespaces: []*corev1.Namespace{testNamespace},
				Secret:     []*corev1.Secret{validSecret},
				Repositories: []*v1alpha1.Repository{
					repoData, repoData1,
				},
			},
			repository: repoData,
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace.Name,
			},
			repoListsByGlobalConf: "",
			wantError:             "",
			wantToken:             "123abcdfrf",
			repositoryID:          []int64{789, 112233},
		},
		{
			name: "repos are listed under repo level configuration but listed repo doesn't exist in namespace",
			tData: testclient.Data{
				Namespaces: []*corev1.Namespace{testNamespace},
				Secret:     []*corev1.Secret{validSecret},
				Repositories: []*v1alpha1.Repository{
					repoData,
				},
			},
			repository: repoData,
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace.Name,
			},
			repoListsByGlobalConf: "",
			wantError:             "failed to scope Github token as repo owner2/repo2 does not exist in namespace pipelinesascode",
			wantToken:             "",
		},
		{
			name: "repo exist and repos are listed under both repo level and global configuration",
			tData: testclient.Data{
				Namespaces: []*corev1.Namespace{testNamespace},
				Secret:     []*corev1.Secret{validSecret},
				Repositories: []*v1alpha1.Repository{
					repoData, repoData1,
				},
			},
			envs: map[string]string{
				"SYSTEM_NAMESPACE": testNamespace.Name,
			},
			repository:            repoData,
			repoListsByGlobalConf: "owner1/repo1",
			wantError:             "",
			wantToken:             "123abcdfrf",
			repositoryID:          []int64{789, 10112, 112233},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tt.tData)
			logger, _ := logger.GetLogger()
			run := &params.Run{
				Clients: clients.Clients{
					Log:            logger,
					PipelineAsCode: stdata.PipelineAsCode,
					Kube:           stdata.Kube,
				},
				Info: info.Info{
					Pac: &info.PacOpts{
						Settings: &settings.Settings{
							SecretGhAppTokenScopedExtraRepos: tt.repoListsByGlobalConf,
						},
					},
				},
			}
			fakeghclient, mux, serverURL, teardown := ghtesthelper.SetupGH()
			defer teardown()
			mux.HandleFunc(fmt.Sprintf("/app/installations/%d/access_tokens", testInstallationID), func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"token": "%s"}`, tempToken)
			})
			tt.envs["PAC_GIT_PROVIDER_TOKEN_APIURL"] = serverURL + "/api/v3"
			envRemove := env.PatchAll(t, tt.envs)
			defer envRemove()

			info := &info.Event{
				Provider: &info.Provider{
					URL: "",
				},
				InstallationID: testInstallationID,
			}

			gvcs := &github.Provider{
				Logger: logger,
				Client: fakeghclient,
			}

			extraRepoInstallIds := map[string]string{"owner/repo": "789", "owner1/repo1": "10112", "owner2/repo2": "112233"}
			for v := range extraRepoInstallIds {
				split := strings.Split(v, "/")
				mux.HandleFunc(fmt.Sprintf("/repos/%s/%s", split[0], split[1]), func(w http.ResponseWriter, r *http.Request) {
					sid := extraRepoInstallIds[fmt.Sprintf("%s/%s", split[0], split[1])]
					_, _ = fmt.Fprintf(w, `{"id": %s}`, sid)
				})
			}
			p := NewPacs(info, gvcs, run, nil, logger)
			token, err := p.scopeTokenToListOfRepos(ctx, tt.repository)
			assert.Equal(t, token, tt.wantToken)
			if err != nil {
				assert.Equal(t, err.Error(), tt.wantError)
			}
			assert.Equal(t, len(gvcs.RepositoryIDs), len(tt.repositoryID))
		})
	}
}
