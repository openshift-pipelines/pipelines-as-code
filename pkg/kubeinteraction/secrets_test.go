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
	secrete := "verysecrete"

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
	event := info.Event{
		Organization: "owner",
		Repository:   "repo",
		URL:          "https://forge/owner/repo",
	}

	tests := []struct {
		name                   string
		targetNS               string
		event                  info.Event
		expectedGitCredentials string
		expectedSecretName     string
	}{
		{
			name:                   "Target secret not there",
			targetNS:               nsNotThere,
			event:                  event,
			expectedGitCredentials: "https://git:verysecrete@forge/owner/repo",
			expectedSecretName:     "pac-git-basic-auth-owner-repo",
		},
		{
			name:                   "Target secret already there",
			targetNS:               nsthere,
			event:                  event,
			expectedGitCredentials: "https://git:verysecrete@forge/owner/repo",
			expectedSecretName:     "pac-git-basic-auth-owner-repo",
		},
		{
			name:                   "Lowercase secrets",
			targetNS:               nsthere,
			event:                  info.Event{Organization: "UPPER", Repository: "CASE", URL: "https://forge/UPPER/CASE"},
			expectedGitCredentials: "https://git:verysecrete@forge/UPPER/CASE",
			expectedSecretName:     "pac-git-basic-auth-upper-case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
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
			err := kint.CreateBasicAuthSecret(ctx, &tt.event, &info.PacOpts{ProviderToken: secrete}, tt.targetNS)
			assert.NilError(t, err)

			slist, err := kint.Run.Clients.Kube.CoreV1().Secrets(tt.targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Assert(t, len(slist.Items) > 0, "Secret has not been created")

			found := false
			for _, s := range slist.Items {
				if s.Name == tt.expectedSecretName {
					found = true
				}
			}
			if !found {
				t.Fatal("failed to create secret ", tt.expectedSecretName)
			}
		})
	}
}

func TestDeleteBasicAuthSecret(t *testing.T) {
	nsthere := "there"

	tdata := testclient.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsthere,
				},
			},
		},
		Secret: []*corev1.Secret{
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
	event := info.Event{
		Organization: "owner",
		Repository:   "repo",
		URL:          "https://forge/owner/repo",
	}
	event2 := info.Event{
		Organization: "owner2",
		Repository:   "repo2",
		URL:          "https://forge/owner/repo2",
	}

	tests := []struct {
		name     string
		targetNS string
		event    info.Event
	}{
		{
			name:     "auth basic secret there",
			targetNS: nsthere,
			event:    event,
		},
		{
			name:     "auth basic secret not there",
			targetNS: nsthere,
			event:    event2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
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
			err := kint.DeleteBasicAuthSecret(ctx, &tt.event, tt.targetNS)
			assert.NilError(t, err)

			slist, err := kint.Run.Clients.Kube.CoreV1().Secrets(tt.targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)

			found := false
			secretName := getBasicAuthSecretName(&tt.event)
			for _, s := range slist.Items {
				if s.Name == secretName {
					found = true
				}
			}
			if found {
				t.Fatal("failed to delete secret ", secretName)
			}
		})
	}
}
