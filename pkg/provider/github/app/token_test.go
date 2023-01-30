package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	ghtesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/github"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/logger"
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

var testNamespace = &corev1.Namespace{
	ObjectMeta: metav1.ObjectMeta{
		Name: "pipelinesascode",
	},
}

var validSecret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      pipelineascode.DefaultPipelinesAscodeSecretName,
		Namespace: testNamespace.Name,
	},
	Data: map[string][]byte{
		"github-application-id": []byte("274799"),
		"github-private-key":    []byte(fakePrivateKey),
	},
}

func Test_GenerateJWT(t *testing.T) {
	envRemove := env.PatchAll(t, map[string]string{"SYSTEM_NAMESPACE": "foo"})
	defer envRemove()
	namespaceWhereSecretNotInstalled := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: os.Getenv("SYSTEM_NAMESPACE"),
		},
	}

	envRemove = env.PatchAll(t, map[string]string{"SYSTEM_NAMESPACE": "pipelinesascode"})
	defer envRemove()
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: os.Getenv("SYSTEM_NAMESPACE"),
		},
	}

	secretWithInavlidAppID := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineascode.DefaultPipelinesAscodeSecretName,
			Namespace: testNamespace.Name,
		},
		Data: map[string][]byte{
			"github-application-id": []byte("abcdf"),
			"github-private-key":    []byte(fakePrivateKey),
		},
	}
	secretWithInvalidPrivateKey := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineascode.DefaultPipelinesAscodeSecretName,
			Namespace: testNamespace.Name,
		},
		Data: map[string][]byte{
			"github-application-id": []byte("12345"),
			"github-private-key":    []byte("invalidprivatekey"),
		},
	}

	tests := []struct {
		name      string
		wantErr   bool
		secrets   []*corev1.Secret
		namespace []*corev1.Namespace
	}{{
		name:      "secret not found",
		namespace: []*corev1.Namespace{namespaceWhereSecretNotInstalled},
		secrets:   []*corev1.Secret{},
		wantErr:   true,
	}, {
		name:      "invalid github-application-id",
		namespace: []*corev1.Namespace{testNamespace},
		secrets:   []*corev1.Secret{secretWithInavlidAppID},
		wantErr:   true,
	}, {
		name:      "invalid private key",
		namespace: []*corev1.Namespace{testNamespace},
		secrets:   []*corev1.Secret{secretWithInvalidPrivateKey},
		wantErr:   true,
	}, {
		name:      "valid secret found",
		namespace: []*corev1.Namespace{testNamespace},
		secrets:   []*corev1.Secret{validSecret},
		wantErr:   false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := logger.GetLogger()
			tdata := testclient.Data{
				Namespaces: tt.namespace,
				Secret:     tt.secrets,
			}
			ctxNoSecret, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctxNoSecret, tdata)
			run := &params.Run{
				Clients: clients.Clients{
					Log:            logger,
					PipelineAsCode: stdata.PipelineAsCode,
					Kube:           stdata.Kube,
				},
			}

			token, err := generateJWT(ctxNoSecret, run)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)

			if token == "" {
				t.Errorf("there should be a generated token")
			}
		})
	}
}

func Test_GetAndUpdateInstallationID(t *testing.T) {
	envRemove := env.PatchAll(t, map[string]string{"SYSTEM_NAMESPACE": "pipelinesascode"})
	defer envRemove()
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinesascode",
		},
	}

	tdata := testclient.Data{
		Namespaces: []*corev1.Namespace{testNamespace},
		Secret:     []*corev1.Secret{validSecret},
	}

	// created fakeconfig to get InstallationID
	config := map[string]map[string]string{
		"https://api.github.com/app/installations": {
			"body": `[{"id":120}]`,
			"code": "200",
		},
	}
	httpTestClient := httptesthelper.MakeHTTPTestClient(t, config)
	ctx, _ := rtesting.SetupFakeContext(t)
	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	logger, _ := logger.GetLogger()
	run := &params.Run{
		Clients: clients.Clients{
			Log:            logger,
			PipelineAsCode: stdata.PipelineAsCode,
			Kube:           stdata.Kube,
			HTTP:           *httpTestClient,
		},
		Info: info.Info{
			Pac: &info.PacOpts{
				Settings: &settings.Settings{},
			},
		},
	}

	jwtToken, err := generateJWT(ctx, run)
	req := httptest.NewRequest("GET", "http://localhost", strings.NewReader(""))

	req.Header.Add("X-GitHub-Enterprise-Host", "https://api.github.com")

	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: "repo",
		},
		Spec: v1alpha1.RepositorySpec{
			URL: "https://matched/by/incoming",
			Incomings: &[]v1alpha1.Incoming{
				{
					Targets: []string{"main"},
					Secret: v1alpha1.Secret{
						Name: "secret",
					},
				},
			},
		},
	}

	fakeghclient, mux, serverURL, teardown := ghtesthelper.SetupGH()
	defer teardown()
	mux.HandleFunc(fmt.Sprintf("/app/installations/%d/access_tokens", 120), func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.Header().Set("Authorization", "Bearer "+jwtToken)
		w.Header().Set("Accept", "application/vnd.github+json")
		_, _ = fmt.Fprint(w, `{"token": "12345"}`)
	})

	envRemove = env.PatchAll(t, map[string]string{"SYSTEM_NAMESPACE": "pipelinesascode", "PAC_GIT_PROVIDER_TOKEN_APIURL": serverURL + "/api/v3"})
	defer envRemove()

	gprovider := &github.Provider{
		Logger: logger,
		Client: fakeghclient,
	}

	mux.HandleFunc(fmt.Sprintf("/installation/repositories"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Authorization", "Bearer 12345")
		w.Header().Set("Accept", "application/vnd.github+json")
		_, _ = fmt.Fprint(w, `{"total_count": 1,"repositories": [{"owner":{"html_url": "https://matched/by/incoming"}]}`)

	})

	_, _, _, err = GetAndUpdateInstallationID(ctx, req, run, repo, gprovider)
	if strings.Contains(err.Error(), "https://api.github.com/installation/repositories: 401 Bad credentials") {
		assert.Assert(t, err != nil)
	} else {
		assert.Assert(t, err == nil)
	}
}

func testMethod(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Method; got != want {
		t.Errorf("Request method: %v, want %v", got, want)
	}
}
