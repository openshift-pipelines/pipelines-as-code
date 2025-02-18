package gitea

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/opscomments"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea/test"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestProvider_CreateStatus(t *testing.T) {
	type args struct {
		event      *info.Event
		statusOpts provider.StatusOpts
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test with success conclusion",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Conclusion: "success",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with failure conclusion",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Conclusion: "failure",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with pending conclusion",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Conclusion: "pending",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with neutral conclusion",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Conclusion: "neutral",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with in_progress status",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Status: "in_progress",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with onpr",
			args: args{
				event: &info.Event{},
				statusOpts: provider.StatusOpts{
					Status:          "in_progress",
					PipelineRunName: "mypr",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with ok-to-test event",
			args: args{
				event: &info.Event{EventType: triggertype.OkToTest.String()},
				statusOpts: provider.StatusOpts{
					Status:          "in_progress",
					PipelineRunName: "mypr",
				},
			},
			wantErr: false,
		},
		{
			name: "Test with oncomment event",
			args: args{
				event: &info.Event{EventType: opscomments.OkToTestCommentEventType.String()},
				statusOpts: provider.StatusOpts{
					Status:          "in_progress",
					PipelineRunName: "mypr",
				},
			},
			wantErr: false,
		},
		{
			name: "Test status_text",
			args: args{
				event: &info.Event{EventType: triggertype.PullRequest.String()},
				statusOpts: provider.StatusOpts{
					Status:          "in_progress",
					PipelineRunName: "mypr",
					Text:            "mytext",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, teardown := tgitea.Setup(t)
			defer teardown()
			run := params.New()
			p := &Provider{
				giteaClient: fakeclient, // Set this to a valid client for the tests where wantErr is false
				run:         run,
				pacInfo: &info.PacOpts{
					Settings: settings.Settings{
						ApplicationName: settings.PACApplicationNameDefaultValue,
					},
				},
			}
			tt.args.event.Organization = "myorg"
			tt.args.event.Repository = "myrepo"

			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/issues/0/comments", tt.args.event.Organization, tt.args.event.Repository), func(rw http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(rw, `{"state":"success"}`)
			})
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/statuses/", tt.args.event.Organization, tt.args.event.Repository), func(rw http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(rw, `{"state":"success"}`)
			})
			if err := p.CreateStatus(context.Background(), tt.args.event, tt.args.statusOpts); (err != nil) != tt.wantErr {
				t.Errorf("Provider.CreateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProvider_GetFiles(t *testing.T) {
	type args struct {
		runevent *info.Event
	}
	tests := []struct {
		name         string
		args         args
		changedFiles string
		want         changedfiles.ChangedFiles
		wantErr      bool
	}{
		{
			name: "pull_request",
			args: args{
				runevent: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					TriggerTarget:     "pull_request",
				},
			},
			want: changedfiles.ChangedFiles{
				All: []string{
					"added.txt",
					"deleted.txt",
					"modified.txt",
					"renamed.txt",
				},
				Added: []string{
					"added.txt",
				},
				Deleted:  []string{"deleted.txt"},
				Modified: []string{"modified.txt"},
				Renamed:  []string{"renamed.txt"},
			},
			changedFiles: `[{"filename":"added.txt","status":"added"},{"filename":"deleted.txt","status":"deleted"},{"filename":"modified.txt","status":"changed"},{"filename":"renamed.txt","status":"renamed"}]`,
		},
		{
			name: "push",
			args: args{
				runevent: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: -1,
					TriggerTarget:     "push",
					Request: &info.Request{
						Payload: []byte(`{"ref":"refs/heads/main","commits":[{"added":["added.txt"],"removed":["deleted.txt"],"modified":["modified.txt"]},{"added":[".tekton/pullrequest.yaml",".tekton/push.yaml"],"removed":[],"modified":[]}]}`),
					},
				},
			},
			want: changedfiles.ChangedFiles{
				All: []string{
					".tekton/pullrequest.yaml",
					".tekton/push.yaml",
					"added.txt",
					"deleted.txt",
					"modified.txt",
				},
				Added: []string{
					".tekton/pullrequest.yaml",
					".tekton/push.yaml",
					"added.txt",
				},
				Deleted:  []string{"deleted.txt"},
				Modified: []string{"modified.txt"},
				// Renamed:  []string{"renamed.txt"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, teardown := tgitea.Setup(t)
			defer teardown()

			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/pulls/%d/files", tt.args.runevent.Organization, tt.args.runevent.Repository, tt.args.runevent.PullRequestNumber), func(rw http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(rw, tt.changedFiles)
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			repo := &v1alpha1.Repository{Spec: v1alpha1.RepositorySpec{
				Settings: &v1alpha1.Settings{},
			}}
			gprovider := Provider{
				giteaClient: fakeclient,
				repo:        repo,
				Logger:      logger,
			}

			got, err := gprovider.GetFiles(ctx, tt.args.runevent)

			if (err != nil) != tt.wantErr {
				t.Errorf("Provider.GetFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			sort.Strings(got.All)
			sort.Strings(tt.want.All)

			sort.Strings(got.Added)
			sort.Strings(tt.want.Added)

			sort.Strings(got.Deleted)
			sort.Strings(tt.want.Deleted)

			sort.Strings(got.Modified)
			sort.Strings(tt.want.Modified)

			sort.Strings(got.Renamed)
			sort.Strings(tt.want.Renamed)
			if !reflect.DeepEqual(got.All, tt.want.All) {
				t.Errorf("Provider.GetFiles() All = %v, want %v", got.All, tt.want.All)
			}
			if !reflect.DeepEqual(got.Added, tt.want.Added) {
				t.Errorf("Provider.GetFiles() Added = %v, want %v", got.Added, tt.want.Added)
			}
			if !reflect.DeepEqual(got.Deleted, tt.want.Deleted) {
				t.Errorf("Provider.GetFiles() Deleted = %v, want %v", got.Deleted, tt.want.Deleted)
			}
			if !reflect.DeepEqual(got.Modified, tt.want.Modified) {
				t.Errorf("Provider.GetFiles() Modified = %v, want %v", got.Modified, tt.want.Modified)
			}
			if !reflect.DeepEqual(got.Renamed, tt.want.Renamed) {
				t.Errorf("Provider.GetFiles() Renamed = %v, want %v", got.Renamed, tt.want.Renamed)
			}
		})
	}
}

func TestProvider_CreateStatusCommit(t *testing.T) {
	type args struct {
		event   *info.Event
		pacopts *info.PacOpts
		status  provider.StatusOpts
	}
	tests := []struct {
		name                            string
		args                            args
		wantErr                         bool
		wantCommentJSON, wantStatusJSON string
	}{
		{
			name: "success",
			args: args{
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					TriggerTarget:     "pull_request",
					SHA:               "123456",
				},
				status: provider.StatusOpts{
					Conclusion: "neutral",
				},
			},
			wantStatusJSON: `{"state":"success","target_url":"","description":"","context":"myapp"}`,
		},
		{
			name: "pending",
			args: args{
				status: provider.StatusOpts{
					Conclusion: "pending",
					Title:      "Pipeline run for myapp has been triggered",
				},
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					TriggerTarget:     "pull_request",
					SHA:               "123456",
				},
			},
			wantStatusJSON: `{"state":"pending","target_url":"","description":"Pipeline run for myapp has been triggered","context":"myapp"}`,
		},
		{
			name: "pending from status",
			args: args{
				status: provider.StatusOpts{
					Status: "in_progress",
					Title:  "Pipeline run for myapp has been triggered",
				},
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					TriggerTarget:     "pull_request",
					SHA:               "123456",
				},
			},
			wantStatusJSON: `{"state":"pending","target_url":"","description":"Pipeline run for myapp has been triggered","context":"myapp"}`,
		},
		{
			name: "ok-to-test",
			args: args{
				status: provider.StatusOpts{
					Conclusion: "pending",
					Title:      "Pipeline run for myapp has been triggered",
					Text:       "time to get started",
				},
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					EventType:         triggertype.OkToTest.String(),
					SHA:               "123456",
				},
			},
			wantStatusJSON:  `{"state":"pending","target_url":"","description":"Pipeline run for myapp has been triggered","context":"myapp"}`,
			wantCommentJSON: `{"body":"\ntime to get started"}`,
		},
		{
			name: "cancel",
			args: args{
				status: provider.StatusOpts{
					Conclusion: "pending",
					Title:      "Pipeline run for myapp has been triggered",
					Text:       "time to get started",
				},
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					EventType:         triggertype.Cancel.String(),
					SHA:               "123456",
				},
			},
			wantStatusJSON:  `{"state":"pending","target_url":"","description":"Pipeline run for myapp has been triggered","context":"myapp"}`,
			wantCommentJSON: `{"body":"\ntime to get started"}`,
		},
		{
			name: "retest",
			args: args{
				status: provider.StatusOpts{
					Conclusion: "pending",
					Title:      "Pipeline run for myapp has been triggered",
					Text:       "time to get started",
				},
				pacopts: &info.PacOpts{Settings: settings.Settings{
					ApplicationName: "myapp",
				}},
				event: &info.Event{
					Organization:      "myorg",
					Repository:        "myrepo",
					PullRequestNumber: 1,
					EventType:         triggertype.Retest.String(),
					SHA:               "123456",
				},
			},
			wantStatusJSON:  `{"state":"pending","target_url":"","description":"Pipeline run for myapp has been triggered","context":"myapp"}`,
			wantCommentJSON: `{"body":"\ntime to get started"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, teardown := tgitea.Setup(t)
			defer teardown()

			// Mock the CreateStatus API
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/statuses/%s", tt.args.event.Organization, tt.args.event.Repository, tt.args.event.SHA), func(rw http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(rw, "Failed to read request body", http.StatusInternalServerError)
					return
				}

				if res := cmp.Diff(string(tt.wantStatusJSON), string(body)); res != "" {
					t.Errorf("Received: %s Diff %s:", string(body), res)
				}

				_, _ = rw.Write([]byte(`{"state":"success"}`))
			})

			// Mock the CreateIssueComment API
			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", tt.args.event.Organization, tt.args.event.Repository, tt.args.event.PullRequestNumber), func(rw http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(rw, "Failed to read request body", http.StatusInternalServerError)
					return
				}

				if res := cmp.Diff(string(tt.wantCommentJSON), string(body)); res != "" {
					t.Errorf("Received: %s Diff %s:", string(body), res)
				}
				_, _ = rw.Write([]byte(`{"body":"Pipeline run for myapp has been triggered"}`))
			})

			v := &Provider{
				giteaClient: fakeclient,
			}

			if err := v.createStatusCommit(tt.args.event, tt.args.pacopts, tt.args.status); (err != nil) != tt.wantErr {
				t.Errorf("Provider.createStatusCommit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTektonDir(t *testing.T) {
	testGetTektonDir := []struct {
		treepath             string
		event                *info.Event
		name                 string
		expectedString       string
		provenance           string
		filterMessageSnippet string
		wantErr              string
	}{
		{
			name: "test with badly formatted yaml",
			event: &info.Event{
				Organization: "tekton",
				Repository:   "cat",
				SHA:          "123",
			},
			treepath: "testdata/tree/badyaml",
			wantErr:  "error unmarshalling yaml file badyaml.yaml: yaml: line 2: did not find expected key",
		},
	}
	for _, tt := range testGetTektonDir {
		t.Run(tt.name, func(t *testing.T) {
			observer, _ := zapobserver.New(zap.InfoLevel)
			fakelogger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			fakeclient, mux, teardown := tgitea.Setup(t)
			defer teardown()
			gvcs := Provider{
				giteaClient: fakeclient,
				Logger:      fakelogger,
			}
			if tt.provenance == "default_branch" {
				tt.event.SHA = tt.event.DefaultBranch
			} else {
				shaDir := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.treepath)))
				tt.event.SHA = shaDir
			}

			tgitea.SetupGitTree(t, mux, tt.treepath, tt.event, false)
			got, err := gvcs.GetTektonDir(ctx, tt.event, ".tekton", tt.provenance)
			if tt.wantErr != "" {
				assert.Assert(t, err != nil, "we should have get an error here")
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, strings.Contains(got, tt.expectedString), "expected %s, got %s", tt.expectedString, got)
		})
	}
}

func TestCreateComment(t *testing.T) {
	tests := []struct {
		name          string
		event         *info.Event
		commit        string
		updateMarker  string
		mockResponses map[string]func(rw http.ResponseWriter, _ *http.Request)
		wantErr       string
		clientNil     bool
	}{
		{
			name:      "nil client error",
			clientNil: true,
			event:     &info.Event{PullRequestNumber: 123},
			wantErr:   "no gitea client has been initialized",
		},
		{
			name:    "not a pull request error",
			event:   &info.Event{PullRequestNumber: 0},
			wantErr: "create comment only works on pull requests",
		},
		{
			name:         "create new comment",
			event:        &info.Event{Organization: "org", Repository: "repo", PullRequestNumber: 123},
			commit:       "New Comment",
			updateMarker: "",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/repos/org/repo/issues/123/comments": func(rw http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, http.MethodPost)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
		{
			name:         "update existing comment",
			event:        &info.Event{Organization: "org", Repository: "repo", PullRequestNumber: 123},
			commit:       "Updated Comment",
			updateMarker: "MARKER",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/repos/org/repo/issues/123/comments": func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						fmt.Fprint(rw, `[{"id": 555, "body": "MARKER"}]`)
						return
					}
				},
				"/repos/org/repo/issues/comments/555": func(rw http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, "PATCH")
					rw.WriteHeader(http.StatusOK)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
		{
			name:         "no matching comment creates new",
			event:        &info.Event{Organization: "org", Repository: "repo", PullRequestNumber: 123},
			commit:       "New Comment",
			updateMarker: "MARKER",
			mockResponses: map[string]func(rw http.ResponseWriter, _ *http.Request){
				"/repos/org/repo/issues/123/comments": func(rw http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						fmt.Fprint(rw, `[{"id": 555, "body": "NO_MATCH"}]`)
						return
					}
					assert.Equal(t, r.Method, http.MethodPost)
					rw.WriteHeader(http.StatusCreated)
					fmt.Fprint(rw, `{}`)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeclient, mux, teardown := tgitea.Setup(t)
			defer teardown()

			if tt.clientNil {
				p := &Provider{}
				err := p.CreateComment(context.Background(), tt.event, tt.commit, tt.updateMarker)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			for endpoint, handler := range tt.mockResponses {
				mux.HandleFunc(endpoint, handler)
			}

			p := &Provider{giteaClient: fakeclient}
			err := p.CreateComment(context.Background(), tt.event, tt.commit, tt.updateMarker)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
