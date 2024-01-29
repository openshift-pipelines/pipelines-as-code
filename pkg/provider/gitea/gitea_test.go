package gitea

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/gitea/test"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	rtesting "knative.dev/pkg/reconciler/testing"
)

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

			mux.HandleFunc(fmt.Sprintf("/repos/%s/%s/pulls/%d/files", tt.args.runevent.Organization, tt.args.runevent.Repository, tt.args.runevent.PullRequestNumber), func(rw http.ResponseWriter, r *http.Request) {
				fmt.Fprint(rw, tt.changedFiles)
			})
			ctx, _ := rtesting.SetupFakeContext(t)
			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			repo := &v1alpha1.Repository{Spec: v1alpha1.RepositorySpec{
				Settings: &v1alpha1.Settings{},
			}}
			gprovider := Provider{
				Client: fakeclient,
				repo:   repo,
				Logger: logger,
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
