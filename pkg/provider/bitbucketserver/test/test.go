package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketserver/types"
	"gotest.tools/v3/assert"
)

var (
	defaultAPIURL = "/api/1.0"
	buildAPIURL   = "/build-status/1.0"
)

func SetupBBServerClient(ctx context.Context) (*bbv1.APIClient, *http.ServeMux, func(), string) {
	mux := http.NewServeMux()
	apiHandler := http.NewServeMux()
	apiHandler.Handle(defaultAPIURL+"/", http.StripPrefix(defaultAPIURL, mux))
	apiHandler.Handle(buildAPIURL+"/", http.StripPrefix(buildAPIURL, mux))
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

	cfg := bbv1.NewConfiguration(server.URL)
	cfg.HTTPClient = server.Client()
	client := bbv1.NewAPIClient(ctx, cfg)
	return client, mux, tearDown, server.URL
}

func MuxCreateComment(t *testing.T, mux *http.ServeMux, event *info.Event, expectedCommentSubstr string, prID int) {
	assert.Assert(t, event.Event != nil)

	path := fmt.Sprintf("/projects/%s/repos/%s/pull-requests/%d/comments", event.Organization, event.Repository, prID)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		cso := &bbv1.Comment{}
		bit, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(bit, cso)
		assert.NilError(t, err)
		if expectedCommentSubstr != "" {
			assert.Assert(t, strings.Contains(cso.Text, expectedCommentSubstr), "comment: %s doesn't have: %s",
				cso.Text, expectedCommentSubstr)
		}

		fmt.Fprintf(rw, "{}")
	})
}

// MakeEvent should we try to reflect? or json.Marshall? may be better ways, right?
func MakeEvent(event *info.Event) *info.Event {
	if event == nil {
		event = info.NewEvent()
	}
	rev := event
	if event.Provider == nil {
		rev.Provider = &info.Provider{}
	}
	if rev.HeadBranch == "" {
		rev.HeadBranch = "pr"
	}
	if rev.BaseBranch == "" {
		rev.BaseBranch = "main"
	}
	if rev.SHA == "" {
		rev.SHA = "1234"
	}
	if rev.Organization == "" {
		rev.Organization = "owner"
	}
	if rev.Repository == "" {
		rev.Repository = "repo"
	}
	if rev.AccountID == "" {
		rev.AccountID = "accountid"
	}
	if rev.DefaultBranch == "" {
		rev.DefaultBranch = "main"
	}
	if rev.Sender == "" {
		rev.Sender = "sender"
	}
	if rev.Event == nil {
		rev.Event = &types.PullRequestEvent{
			PullRequest: bbv1.PullRequest{ID: 666},
		}
	}
	return rev
}

func MuxDirContent(t *testing.T, mux *http.ServeMux, event *info.Event, testDir, targetDirName string) {
	files, err := os.ReadDir(testDir)
	if err != nil {
		// no error just disappointed
		return
	}
	filenames := make([]string, 0, len(files))
	filecontents := map[string]string{}
	for _, value := range files {
		path := filepath.Join(targetDirName, value.Name())
		filenames = append(filenames, value.Name())

		fpath := filepath.Join(testDir, value.Name())
		if info, err := os.Stat(fpath); err == nil && info.IsDir() {
			continue
		}
		content, err := os.ReadFile(fpath)
		assert.NilError(t, err)
		filecontents[path] = string(content)
	}

	MuxListDir(t, mux, event, targetDirName, filenames)
	MuxFiles(t, mux, event, event.HeadBranch, targetDirName, filecontents)
}

func MuxCommitInfo(t *testing.T, mux *http.ServeMux, event *info.Event, commit bbv1.Commit) {
	path := fmt.Sprintf("/projects/%s/repos/%s/commits/%s", event.Organization, event.Repository, event.SHA)

	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		b, err := json.Marshal(commit)
		assert.NilError(t, err)
		fmt.Fprint(rw, string(b))
	})
}

func MuxDefaultBranch(t *testing.T, mux *http.ServeMux, event *info.Event, defaultBranch, latestCommit string) {
	path := fmt.Sprintf("/projects/%s/repos/%s/branches/default", event.Organization, event.Repository)
	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		resp := &bbv1.Branch{
			LatestCommit: latestCommit,
			DisplayID:    defaultBranch,
		}
		b, err := json.Marshal(resp)
		assert.NilError(t, err)
		fmt.Fprint(rw, string(b))
	})
}

func MuxFiles(_ *testing.T, mux *http.ServeMux, event *info.Event, _, targetDirName string, filescontents map[string]string) {
	for filename := range filescontents {
		path := fmt.Sprintf("/projects/%s/repos/%s/raw/%s", event.Organization, event.Repository, filename)
		mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
			// delete everything until the targetDirName in string this is
			// fragile, so we do the filepath.Base if we can't mark by targetDirName
			s := r.URL.Path[strings.LastIndex(r.URL.Path, targetDirName):]
			if s == "" {
				s = filepath.Base(r.URL.Path)
			}
			fmt.Fprint(rw, filescontents[s])
		})
	}
}

func MuxListDir(t *testing.T, mux *http.ServeMux, event *info.Event, path string, files []string) {
	url := fmt.Sprintf("/projects/%s/repos/%s/files/%s", event.Organization, event.Repository, path)
	mux.HandleFunc(url, func(rw http.ResponseWriter, r *http.Request) {
		pagedfiles := files[0 : len(files)/2]
		isLastPage := false
		nextPageStart := 1
		start := 0
		if r.URL.Query().Get("start") != "" {
			pagedfiles = files[len(files)/2:]
			isLastPage = true
			nextPageStart = 0
			start = 1
		}

		resp := map[string]interface{}{
			"start":         start,
			"isLastPage":    isLastPage,
			"values":        pagedfiles,
			"size":          len(files) / 2,
			"nextPageStart": nextPageStart,
		}
		b, err := json.Marshal(resp)
		assert.NilError(t, err)
		fmt.Fprint(rw, string(b))
	})
}

func MuxCreateAndTestCommitStatus(t *testing.T, mux *http.ServeMux, event *info.Event, expectedDescSubstr string, expStatus provider.StatusOpts) {
	path := fmt.Sprintf("/commits/%s", event.SHA)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		cso := &bbv1.BuildStatus{}
		bit, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(bit, cso)
		assert.NilError(t, err)

		if expStatus.DetailsURL != "" {
			assert.Equal(t, expStatus.DetailsURL, cso.Url)
		}
		if expectedDescSubstr != "" {
			assert.Assert(t, strings.Contains(cso.Description, expectedDescSubstr),
				"description: %s doesn't have: %s", cso.Description, expectedDescSubstr)
		}

		fmt.Fprintf(rw, "{}")
	})
}

func MuxProjectMemberShip(t *testing.T, mux *http.ServeMux, event *info.Event, userperms []*bbv1.UserPermission) {
	path := fmt.Sprintf("/projects/%s/permissions/users", event.Organization)
	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		if userperms == nil {
			fmt.Fprintf(rw, "{\"values\": []}")
		}
		resp := map[string]interface{}{
			"values": userperms,
		}
		b, err := json.Marshal(resp)
		assert.NilError(t, err)

		fmt.Fprint(rw, string(b))
	})
}

func MuxRepoMemberShip(t *testing.T, mux *http.ServeMux, event *info.Event, userperms []*bbv1.UserPermission) {
	path := fmt.Sprintf("/projects/%s/repos/%s/permissions/users", event.Organization, event.Repository)
	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		if userperms == nil {
			fmt.Fprintf(rw, "{\"values\": []}")
		}
		resp := map[string]interface{}{
			"values": userperms,
		}
		b, err := json.Marshal(resp)
		assert.NilError(t, err)
		fmt.Fprint(rw, string(b))
	})
}

func MuxPullRequestActivities(t *testing.T, mux *http.ServeMux, event *info.Event, prNumber int, activities []*bbv1.Activity) {
	path := fmt.Sprintf("/projects/%s/repos/%s/pull-requests/%d/activities", event.Organization, event.Repository, prNumber)
	mux.HandleFunc(path, func(rw http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"values": activities,
		}
		b, err := json.Marshal(resp)
		assert.NilError(t, err)

		fmt.Fprint(rw, string(b))
	})
}

func MakePREvent(event *info.Event, comment string) *types.PullRequestEvent {
	iii, _ := strconv.Atoi(event.AccountID)

	pr := &types.PullRequestEvent{
		Actor: bbv1.UserWithLinks{ID: iii, Name: event.Sender},
		PullRequest: bbv1.PullRequest{
			ID: 1,
			ToRef: bbv1.PullRequestRef{
				Repository: bbv1.Repository{
					Project: &bbv1.Project{Key: event.Organization},
					Name:    event.Repository,
					Links: &struct {
						Clone []bbv1.CloneLink `json:"clone,omitempty"`
						Self  []bbv1.SelfLink  `json:"self,omitempty"`
					}{
						Self: []bbv1.SelfLink{
							{
								Href: event.URL,
							},
						},
						Clone: []bbv1.CloneLink{{Href: event.URL}},
					},
				},
				DisplayID:    "base",
				LatestCommit: "abcd",
			},
			FromRef: bbv1.PullRequestRef{
				DisplayID:    "head",
				LatestCommit: event.SHA,
				Repository: bbv1.Repository{
					Project: &bbv1.Project{Key: event.Organization},
					Name:    event.Repository,
					Links: &struct {
						Clone []bbv1.CloneLink `json:"clone,omitempty"`
						Self  []bbv1.SelfLink  `json:"self,omitempty"`
					}{
						Self: []bbv1.SelfLink{
							{
								Href: event.HeadURL,
							},
						},
						Clone: []bbv1.CloneLink{
							{
								Name: "http",
								Href: event.CloneURL,
							},
						},
					},
				},
			},
		},
	}
	if comment != "" {
		pr.Comment = bbv1.ActivityComment{
			Text: comment,
		}
	}
	return pr
}

func MakePushEvent(event *info.Event) *types.PushRequestEvent {
	iii, _ := strconv.Atoi(event.AccountID)

	return &types.PushRequestEvent{
		Actor: bbv1.UserWithLinks{ID: iii, Name: event.Sender},
		Repository: bbv1.Repository{
			Project: &bbv1.Project{
				Key: event.Organization,
			},
			Slug: event.Repository,
			Links: &struct {
				Clone []bbv1.CloneLink `json:"clone,omitempty"`
				Self  []bbv1.SelfLink  `json:"self,omitempty"`
			}{
				Clone: []bbv1.CloneLink{
					{
						Name: "http",
						Href: event.CloneURL,
					},
				},
				Self: []bbv1.SelfLink{
					{
						Href: event.URL,
					},
				},
			},
		},
		Changes: []types.PushRequestEventChange{
			{
				ToHash: event.SHA,
				RefID:  "base",
			},
		},
	}
}
