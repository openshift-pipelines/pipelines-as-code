package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"code.gitea.io/sdk/gitea"
	"gotest.tools/v3/assert"
)

var (
	defaultAPIURL   = "/api/v1"
	giteaMinVersion = "1.17.0"
)

func Setup(t *testing.T) (*gitea.Client, *http.ServeMux, func()) {
	mux := http.NewServeMux()
	apiHandler := http.NewServeMux()
	apiHandler.Handle(defaultAPIURL+"/", http.StripPrefix(defaultAPIURL, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, `{"version": "%s"}`, giteaMinVersion)
	})
	//
	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	tearDown := func() {
		server.Close()
	}

	client, err := gitea.NewClient(server.URL)
	assert.NilError(t, err)
	return client, mux, tearDown
}
