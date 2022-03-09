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

type MMEvent struct {
	Username, DefaultBranch, URL, SHA, SHAurl, SHAtitle, Headbranch, Basebranch string
	UserID, MRID, TargetProjectID, SourceProjectID                              int
	PathWithNameSpace                                                           string
}

func MakeMergeEvent(mm MMEvent) string {
	return fmt.Sprintf(`{
    "user": {
        "id": %d,
        "username": "%s"
    },
    "project": {
        "id": %d,
        "web_url": "%s",
        "default_branch": "%s"
    },
    "object_attributes": {
        "iid": %d,
        "source_project_id": %d,
        "title": "%s",
        "source_branch": "%s",
        "target_branch": "%s",
        "last_commit": {
            "id": "%s",
            "url": "%s"
        },
		"target": {
			"path_with_namespace": "%s"
		}
    }
}`, mm.UserID, mm.Username, mm.TargetProjectID, mm.URL, mm.DefaultBranch, mm.MRID,
		mm.SourceProjectID, mm.SHAtitle, mm.Headbranch, mm.Basebranch, mm.SHA, mm.SHAurl, mm.PathWithNameSpace)
}
