package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/xanzy/go-gitlab"
	"gotest.tools/v3/assert"
)

var defaultAPIURL = "/api/v4"

func Setup(ctx context.Context, t *testing.T) (*gitlab.Client, *http.ServeMux, func()) {
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

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	tearDown := func() {
		server.Close()
	}

	client, err := gitlab.NewClient("token", gitlab.WithBaseURL(server.URL))
	assert.NilError(t, err)
	return client, mux, tearDown
}

func MuxNotePost(t *testing.T, mux *http.ServeMux, projectNumber int, mrID int, catchStr string) {
	path := fmt.Sprintf("/projects/%d/merge_requests/%d/notes", projectNumber, mrID)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		bit, _ := ioutil.ReadAll(r.Body)
		s := string(bit)
		if catchStr != "" {
			assert.Assert(t, strings.Contains(s, catchStr), "%s is not in %s", catchStr, s)
		}
		fmt.Fprintf(rw, "{}")
	})
}
