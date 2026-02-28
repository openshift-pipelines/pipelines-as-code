package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/* ---------------- helpers ---------------- */

func newTestProvider(baseURL string) *Provider {
	client := github.NewClient(nil)
	parsed, _ := url.Parse(baseURL)
	client.BaseURL = parsed

	observer, _ := zapobserver.New(zap.DebugLevel)
	logger := zap.New(observer).Sugar()

	return &Provider{
		ghClient:     client,
		Logger:       logger,
		providerName: "github",
		triggerEvent: "push",
		repo: &v1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
				Name:      "test-repo",
			},
		},
	}
}

func newTestGraphQLClient(t *testing.T, baseURL string) *graphQLClient {
	t.Helper()
	provider := newTestProvider(baseURL)
	c, err := newGraphQLClient(provider)
	assert.NilError(t, err)
	return c
}

func withServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}

func graphqlOK(repo map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"repository": repo},
		})
	}
}

func graphqlStatus(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}
}

func mockRepo(n int) map[string]any {
	repo := make(map[string]any)
	for i := range n {
		repo[fmt.Sprintf("file%d", i)] = map[string]any{
			"text": fmt.Sprintf("content-%d", i),
		}
	}
	return repo
}

/* ---------------- tests ---------------- */

func TestBuildGraphQLEndpoint(t *testing.T) {
	cases := []struct {
		name string
		base string
		want string
	}{
		{"public", "https://api.github.com", "https://api.github.com/graphql"},
		{"public slash", "https://api.github.com/", "https://api.github.com/graphql"},
		{"ghe v3", "https://ghe/x/api/v3", "https://ghe/x/api/graphql"},
		{"ghe v3 slash", "https://ghe/x/api/v3/", "https://ghe/x/api/graphql"},
		{"ghe root", "https://ghe", "https://ghe/api/graphql"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(nil)
			parsed, _ := url.Parse(tc.base)
			client.BaseURL = parsed

			got, err := buildGraphQLEndpoint(&Provider{ghClient: client})
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(tc.want, got))
		})
	}
}

func TestBuildGraphQLQuery(t *testing.T) {
	cases := []struct {
		name  string
		ref   string
		paths []string
		want  []string
	}{
		{
			name:  "two files",
			ref:   "main",
			paths: []string{"a", "b"},
			want:  []string{"query(", "repository(", "file0:", "file1:"},
		},
		{
			name:  "no files",
			ref:   "main",
			paths: nil,
			want:  []string{"query(", "repository("},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := buildGraphQLQuery(tc.ref, tc.paths)
			for _, s := range tc.want {
				assert.Check(t, strings.Contains(q, s))
			}
		})
	}
}

func TestFetchFilesBatch(t *testing.T) {
	cases := []struct {
		name    string
		paths   []string
		handler http.HandlerFunc
		want    int
		wantErr bool
	}{
		{
			name:    "single batch",
			paths:   []string{"a", "b"},
			handler: graphqlOK(mockRepo(2)),
			want:    2,
		},
		{
			name:    "http error",
			paths:   []string{"a"},
			handler: graphqlStatus(http.StatusNotFound),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/graphql", tc.handler)

			srv := withServer(t, mux)
			c := newTestGraphQLClient(t, srv.URL+"/api/v3/")

			res, err := c.fetchFilesBatch(context.Background(), "o", "r", "main", tc.paths)
			if tc.wantErr {
				assert.Assert(t, err != nil)
				return
			}

			assert.NilError(t, err)
			assert.Check(t, cmp.Len(res, tc.want))
		})
	}
}
