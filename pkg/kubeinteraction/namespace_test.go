package kubeinteraction

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestInteraction_GetNamespace(t *testing.T) {
	tests := []struct {
		name     string
		targetNS string
		logMsg   string
		wantErr  bool
	}{
		{
			name:     "error when namespace is not there",
			targetNS: "not existing",
			logMsg:   "namespace is",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: tt.targetNS,
							// TODO: it's not working ? go figure out why! It doesn't actually create a NS here
						},
					},
				},
			}

			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			observer, _ := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			kint := Interaction{
				Run: &params.Run{
					Clients: clients.Clients{
						Kube: stdata.Kube,
						Log:  fakelogger,
					},
				},
			}

			if err := kint.GetNamespace(ctx, tt.targetNS); (err != nil) != tt.wantErr {
				t.Errorf("GetNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
