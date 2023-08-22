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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestDetectPacInstallation(t *testing.T) {
	testParams := []struct {
		name                  string
		namespace             string
		userProvidedNamespace string
		configMap             *corev1.ConfigMap
		wantInstalled         bool
		wantNamespace         string
		wantError             bool
		errorMsg              string
	}{
		{
			name:          "get configmap in pipeline-as-code namespace",
			namespace:     pacNS,
			configMap:     getConfigMapData(pacNS, "v0.17.2"),
			wantNamespace: pacNS,
			wantInstalled: true,
		}, {
			name:          "get configmap in openshift-pipelines namespace",
			namespace:     "openshift-pipelines",
			configMap:     getConfigMapData("openshift-pipelines", "v0.17.2"),
			wantNamespace: "openshift-pipelines",
			wantInstalled: true,
		}, {
			name:                  "get configmap present in different namespace other than default namespaces",
			namespace:             "test",
			userProvidedNamespace: "test",
			configMap:             getConfigMapData("test", "dev"),
			wantNamespace:         "test",
			wantInstalled:         true,
		}, {
			name:                  "configmap not in default namespace",
			namespace:             "test",
			userProvidedNamespace: "",
			configMap:             getConfigMapData("test", "v0.17.2"),
			wantError:             true,
			errorMsg:              "could not detect Pipelines as Code configmap on the cluster, please specify the namespace in which pac is installed: ConfigMap not found in default namespaces (\"openshift-pipelines\", \"pipelines-as-code\")",
			wantInstalled:         false,
		}, {
			name:                  "configmap not in default namespace with user provided namespace",
			namespace:             "test",
			userProvidedNamespace: "test1",
			configMap:             getConfigMapData("test", "v0.17.2"),
			wantError:             true,
			errorMsg:              "could not detect Pipelines as Code configmap in test1 namespace : configmaps \"pipelines-as-code-info\" not found, please reinstall",
			wantInstalled:         false,
		},
	}
	for _, tp := range testParams {
		t.Run(tp.name, func(t *testing.T) {
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
			if _, err := run.Clients.Kube.CoreV1().ConfigMaps(tp.namespace).Create(ctx, tp.configMap, metav1.CreateOptions{}); err != nil {
				t.Errorf("failed to create configmap: %v", err)
			}
			installed, ns, err := DetectPacInstallation(ctx, tp.userProvidedNamespace, run)
			if err != nil {
				if !tp.wantError {
					t.Errorf("Not expecting error but got: %v", err)
				} else {
					assert.Equal(t, err.Error(), tp.errorMsg)
				}
			} else {
				assert.Equal(t, tp.wantInstalled, installed)
				assert.Equal(t, tp.wantNamespace, ns)
			}
		})
	}
}

func getConfigMapData(namespace, version string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      infoConfigMap,
			Namespace: namespace,
		},
		Data: map[string]string{
			"version": version,
		},
	}
}
