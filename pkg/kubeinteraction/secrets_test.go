package kubeinteraction

import (
	"strings"
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
	event := info.NewEvent()
	event.Organization = "owner"
	event.Repository = "repo"
	event.URL = "https://forge/owner/repo"

	lowercaseEvent := info.NewEvent()
	lowercaseEvent.Organization = "UPPER"
	lowercaseEvent.Repository = "CASE"
	lowercaseEvent.URL = "https://forge/UPPER/CASE"

	tests := []struct {
		name                    string
		targetNS                string
		event                   *info.Event
		expectedGitCredentials  string
		expectedStartSecretName string
		expectedError           bool
	}{
		{
			name:                    "Target secret not there",
			targetNS:                nsNotThere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
		},
		{
			name:                    "Target secret already there",
			targetNS:                nsthere,
			event:                   event,
			expectedGitCredentials:  "https://git:verysecrete@forge/owner/repo",
			expectedStartSecretName: "pac-gitauth-owner-repo",
		},
		{
			name:                    "Lowercase secrets",
			targetNS:                nsthere,
			event:                   lowercaseEvent,
			expectedGitCredentials:  "https://git:verysecrete@forge/UPPER/CASE",
			expectedStartSecretName: "pac-gitauth-upper-case",
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
					},
				},
			}
			tt.event.Provider.Token = secrete
			err := kint.CreateBasicAuthSecret(ctx, fakelogger, tt.event, tt.targetNS, tt.expectedStartSecretName)
			assert.NilError(t, err)

			slist, err := kint.Run.Clients.Kube.CoreV1().Secrets(tt.targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)
			assert.Assert(t, len(slist.Items) > 0, "Secret has not been created")

			found := false
			for _, s := range slist.Items {
				if strings.HasPrefix(s.GetName(), tt.expectedStartSecretName) && s.StringData[".git-credentials"] == tt.expectedGitCredentials {
					found = true
				}
			}
			if !found {
				t.Fatalf("we could not find the secret %s out of secrets created: %+v", tt.expectedStartSecretName, slist.Items)
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

	tests := []struct {
		name     string
		targetNS string
	}{
		{
			name:     "auth basic secret there",
			targetNS: nsthere,
		},
		{
			name:     "auth basic secret not there",
			targetNS: nsthere,
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
					},
				},
			}
			err := kint.DeleteBasicAuthSecret(ctx, fakelogger, "", tt.targetNS)
			assert.NilError(t, err)

			slist, err := kint.Run.Clients.Kube.CoreV1().Secrets(tt.targetNS).List(ctx, metav1.ListOptions{})
			assert.NilError(t, err)

			found := false
			secretName := GetBasicAuthSecretName()
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

func TestGetBasicAuthSecret(t *testing.T) {
	t1 := GetBasicAuthSecretName()
	t2 := GetBasicAuthSecretName()
	assert.Assert(t, t1 != t2)
}
