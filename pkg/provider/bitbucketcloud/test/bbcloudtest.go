package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketcloud/types"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
)

const bbBaseURLPath = "/2.0"

func SetupBBCloudClient(t *testing.T) (*bitbucket.Client, *http.ServeMux, func()) {
	mux := http.NewServeMux()
	apiHandler := http.NewServeMux()
	apiHandler.Handle(bbBaseURLPath+"/", http.StripPrefix(bbBaseURLPath, mux))
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

	restoreEnv := env.PatchAll(t, map[string]string{
		"BITBUCKET_API_BASE_URL": server.URL + bbBaseURLPath,
	})

	tearDown := func() {
		server.Close()
		restoreEnv()
	}

	client := bitbucket.NewBasicAuth("", "")
	client.HttpClient = server.Client()
	return client, mux, tearDown
}

func MuxComments(t *testing.T, mux *http.ServeMux, event *info.Event, comments []types.Comment) {
	assert.Assert(t, event.Event != nil)

	pr, ok := event.Event.(*types.PullRequestEvent)
	assert.Assert(t, ok)
	prID := fmt.Sprintf("%d", pr.PullRequest.ID)
	mux.HandleFunc("/repositories/"+event.Organization+"/"+event.Repository+"/pullrequests/"+prID+"/comments/",
		func(rw http.ResponseWriter, r *http.Request) {
			members := &types.Comments{
				Values: comments,
			}
			b, err := json.Marshal(members)
			assert.NilError(t, err)
			fmt.Fprint(rw, string(b))
		})
}

func MuxOrgMember(t *testing.T, mux *http.ServeMux, event *info.Event, members []types.Member) {
	mux.HandleFunc("/workspaces/"+event.Organization+"/members",
		func(rw http.ResponseWriter, r *http.Request) {
			members := &types.Members{
				Values: members,
			}
			b, err := json.Marshal(members)
			assert.NilError(t, err)
			fmt.Fprint(rw, string(b))
		})
}

func MuxFiles(t *testing.T, mux *http.ServeMux, event *info.Event, filescontents map[string]string) {
	for key := range filescontents {
		target := fmt.Sprintf("/repositories/%s/%s/src/%s", event.Organization, event.Repository, event.SHA)
		mux.HandleFunc(target+"/"+key, func(rw http.ResponseWriter, r *http.Request) {
			s := strings.ReplaceAll(r.URL.String(), target, "")
			s = strings.TrimPrefix(s, "/")
			fmt.Fprint(rw, filescontents[s])
		})
	}
}

func MuxListDir(t *testing.T, mux *http.ServeMux, event *info.Event, dirs map[string][]bitbucket.RepositoryFile) {
	for key, value := range dirs {
		urlp := "/repositories/" + event.Organization + "/" + event.Repository + "/src/" + event.SHA + "/" + key + "/"
		mux.HandleFunc(urlp, func(rw http.ResponseWriter, r *http.Request) {
			dircontents := map[string][]bitbucket.RepositoryFile{
				"values": value,
			}
			b, err := json.Marshal(dircontents)
			assert.NilError(t, err)
			fmt.Fprint(rw, string(b))
		})
	}
}

func MuxCommits(t *testing.T, mux *http.ServeMux, event *info.Event, commits []types.Commit) {
	path := fmt.Sprintf("/repositories/%s/%s/commits/%s", event.Organization, event.Repository, event.SHA)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		dircontents := map[string][]types.Commit{
			"values": commits,
		}
		b, _ := json.Marshal(dircontents)
		fmt.Fprint(rw, string(b))
	})
}

func MuxRepoInfo(t *testing.T, mux *http.ServeMux, event *info.Event, repo *bitbucket.Repository) {
	path := fmt.Sprintf("/repositories/%s/%s", event.Organization, event.Repository)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(repo)
		fmt.Fprint(rw, string(b))
	})
}

func MuxCreateCommitstatus(t *testing.T, mux *http.ServeMux, event *info.Event, expectedDescSubstr string, expStatus provider.StatusOpts) {
	path := fmt.Sprintf("/repositories/%s/%s/commit/%s/statuses/build", event.Organization, event.Repository, event.SHA)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		cso := &bitbucket.CommitStatusOptions{}
		bit, _ := ioutil.ReadAll(r.Body)
		err := json.Unmarshal(bit, cso)
		assert.NilError(t, err)

		if expStatus.DetailsURL != "" {
			assert.Equal(t, expStatus.DetailsURL, cso.Url)
		}

		if expectedDescSubstr != "" {
			assert.Assert(t, strings.Contains(cso.Description, expectedDescSubstr), "description: %s doesn't have: %s", cso.Description, expectedDescSubstr)
		}

		fmt.Fprintf(rw, "{}")
	})
}

func MuxCreateComment(t *testing.T, mux *http.ServeMux, event *info.Event, expectedCommentSubstr string) {
	assert.Assert(t, event.Event != nil)
	prev, ok := event.Event.(*types.PullRequestEvent)
	assert.Assert(t, ok)
	prID := fmt.Sprintf("%d", prev.PullRequest.ID)

	path := fmt.Sprintf("/repositories/%s/%s/pullrequests/%s/comments", event.Organization, event.Repository, prID)
	mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		cso := &types.Comment{}
		bit, _ := ioutil.ReadAll(r.Body)
		err := json.Unmarshal(bit, cso)
		assert.NilError(t, err)
		if expectedCommentSubstr != "" {
			assert.Assert(t, strings.Contains(cso.Content.Raw, expectedCommentSubstr), "comment: %s doesn't have: %s",
				cso.Content.Raw, expectedCommentSubstr)
		}

		fmt.Fprintf(rw, "{}")
	})
}

func MuxDirContent(t *testing.T, mux *http.ServeMux, event *info.Event, testDir string, targetDirName string) {
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		// no error just disapointed
		return
	}

	repofiles := []bitbucket.RepositoryFile{}
	filecontents := map[string]string{}
	for _, value := range files {
		path := filepath.Join(targetDirName, value.Name())
		repofiles = append(repofiles, bitbucket.RepositoryFile{
			Path: path,
		})
		fpath := filepath.Join(testDir, value.Name())
		if info, err := os.Stat(fpath); err == nil && info.IsDir() {
			continue
		}
		content, err := ioutil.ReadFile(fpath)
		assert.NilError(t, err)
		filecontents[path] = string(content)
	}

	MuxListDir(t, mux, event, map[string][]bitbucket.RepositoryFile{
		targetDirName: repofiles,
	})
	MuxFiles(t, mux, event, filecontents)
}

func MakePREvent(accountid, nickname, sha string) types.PullRequestEvent {
	if accountid == "" {
		accountid = "prid"
	}
	if nickname == "" {
		nickname = "prnickname"
	}
	if sha == "" {
		sha = "prchat"
	}
	return types.PullRequestEvent{
		Repository: types.Repository{
			Workspace: types.Workspace{
				Slug: "organization",
			},
			Name: "repo",
			Links: types.Links{
				HTML: types.HTMLLink{
					HRef: "https://notgh.org/organization/repo",
				},
			},
		},
		PullRequest: types.PullRequest{
			Author: types.Author{
				AccountID: accountid,
				Nickname:  nickname,
			},
			Destination: types.Destination{
				Branch: types.Branch{
					Name: "main",
				},
			},
			Source: types.Source{
				Branch: types.Branch{
					Name: "branch",
				},
				Commit: types.Commit{
					Hash:    sha,
					Message: "First Draft",
				},
			},
		},
	}
}

func MakePushEvent(accountid, nickname, sha string) types.PushRequestEvent {
	if accountid == "" {
		accountid = "countlady"
	}
	if nickname == "" {
		nickname = "Fonzie"
	}
	if sha == "" {
		sha = "chatchien"
	}
	return types.PushRequestEvent{
		Actor: types.User{
			AccountID: accountid,
			Nickname:  nickname,
		},
		Push: types.Push{
			Changes: []types.Change{
				{
					New: types.ChangeType{
						Target: types.Commit{
							Hash: sha,
						},
					},
					Old: types.ChangeType{
						Target: types.Commit{
							Hash: "veryold",
						},
					},
				},
			},
		},
		Repository: types.Repository{
			Workspace: types.Workspace{
				Slug: "org",
			},
			Name: "repo",
			Links: types.Links{
				HTML: types.HTMLLink{
					HRef: "https://vavar/repo/org",
				},
			},
		},
	}
}

// MakeEvent should we try to reflect? or json.Marshall? may be better ways, right?
func MakeEvent(event *info.Event) *info.Event {
	if event == nil {
		event = &info.Event{}
	}
	rev := event
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
		rev.Event = &types.PullRequestEvent{PullRequest: types.PullRequest{ID: 666}}
	}
	return rev
}

func MakeMuxedHTTPClient() {
}
