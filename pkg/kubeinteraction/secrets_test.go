package kubeinteraction

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCreateBasicAuthSecret(t *testing.T) {
	nsNotThere := "not_there"
	nsthere := "there"

	ctx, _ := rtesting.SetupFakeContext(t)
	tdata := testclient.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsNotThere,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsthere,
				},
			},
		},
		Secret: []*corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsNotThere,
					Name:      "foo-bar-linux-bar",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsthere,
					Name:      "pac-git-basic-auth-owner-repo",
				},
				StringData: map[string]string{
					".git-credentials": "https://whateveryousayboss",
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
	runevent := info.Event{
		Owner:      "owner",
		Repository: "repo",
		URL:        "https://forge/owner/repo",
	}

	tests := []struct {
		name     string
		targetNS string
	}{
		{
			name:     "Target secret not there",
			targetNS: nsNotThere,
		},
		{
			name:     "Target secret already there",
			targetNS: nsthere,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := kint.CreateBasicAuthSecret(ctx, &runevent, &info.PacOpts{VCSToken: "verysecrete"}, tt.targetNS)
			assert.NilError(t, err)
			slist, err := kint.Run.Clients.Kube.CoreV1().Secrets(tt.targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Assert(t, len(slist.Items) > 0, "Secret has not been created")
			assert.Equal(t, slist.Items[0].Name, "pac-git-basic-auth-owner-repo")
			assert.Equal(t, slist.Items[0].StringData[".git-credentials"], "https://git:verysecrete@forge/owner/repo")
		})
	}
}
