package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-github/v34/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	fakepacclientset "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned/fake"
	informersv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/informers/externalversions/pipelinesascode/v1alpha1"
	fakepacclient "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/client/fake"
	fakerepositoryinformers "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/injection/informers/pipelinesascode/v1alpha1/repository/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Clients struct {
	PipelineAsCode *fakepacclientset.Clientset
}

// Informers holds references to informers which are useful for reconciler tests.
type Informers struct {
	Repository informersv1alpha1.RepositoryInformer
}

type Data struct {
	Repositories []*v1alpha1.Repository
}

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	githubBaseURLPath = "/api-v3"
)

// SetupGH Setup a GitHUB httptest connexion, from go-github test-suit
func SetupGH() (client *github.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(githubBaseURLPath+"/", http.StripPrefix(githubBaseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client = github.NewClient(nil)
	url, _ := url.Parse(server.URL + githubBaseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

// SeedTestData returns Clients and Informers populated with the
// given Data.
// nolint: golint
func SeedTestData(t *testing.T, ctx context.Context, d Data) (Clients, Informers) {
	c := Clients{
		PipelineAsCode: fakepacclient.Get(ctx),
	}
	i := Informers{
		Repository: fakerepositoryinformers.Get(ctx),
	}

	for _, repo := range d.Repositories {
		if err := i.Repository.Informer().GetIndexer().Add(repo); err != nil {
			t.Fatal(err)
		}
		if _, err := c.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(repo.Namespace).Create(ctx, repo, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}

	}

	c.PipelineAsCode.ClearActions()
	return c, i
}
