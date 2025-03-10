/*
Copyright 2021 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package bitbucketdatacenter

import (
	"fmt"
	"testing"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	bbv1test "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/bitbucketdatacenter/test"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestIsAllowed(t *testing.T) {
	ownerAccountID := 1234
	otherAccountID := 6666

	type fields struct {
		projectMembers            []*bbv1.UserPermission
		repoMembers               []*bbv1.UserPermission
		activities                []*bbv1.Activity
		filescontents             map[string]string
		defaultBranchLatestCommit string
		pullRequestNumber         int
	}
	tests := []struct {
		name          string
		event         *info.Event
		fields        fields
		isAllowed     bool
		wantErrSubstr string
	}{
		{
			name:  "allowed/user is owner",
			event: bbv1test.MakeEvent(&info.Event{Sender: "member", AccountID: fmt.Sprintf("%d", ownerAccountID)}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							ID: ownerAccountID,
						},
					},
				},
				pullRequestNumber: 1,
			},
			isAllowed: true,
		},
		{
			name: "allowed/from a comment owner",
			event: bbv1test.MakeEvent(&info.Event{
				AccountID: fmt.Sprintf("%d", otherAccountID),
				Sender:    "NotAllowedAtFirst",
			}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							ID: ownerAccountID,
						},
					},
				},
				activities: []*bbv1.Activity{
					{
						Comment: bbv1.ActivityComment{
							Text: "/ok-to-test",
							Author: bbv1.User{
								ID: ownerAccountID,
							},
						},
					},
				},
				pullRequestNumber: 1,
			},
			isAllowed: true,
		},
		{
			name: "allowed/from owner file who is not part of workspace",
			event: bbv1test.MakeEvent(&info.Event{
				AccountID:     fmt.Sprintf("%d", otherAccountID),
				DefaultBranch: "default",
			}),
			fields: fields{
				defaultBranchLatestCommit: "defaultlatestcommit",
				activities: []*bbv1.Activity{
					{
						Comment: bbv1.ActivityComment{
							Text: "/ok-to-test",
							Author: bbv1.User{
								ID: 15551,
							},
						},
					},
				},
				filescontents: map[string]string{
					"OWNERS": "---\n approvers:\n  - 15551\n",
				},
			},
			isAllowed: true,
		},
		{
			name: "disallowed/from an ownerfile that has nothing to do with sender",
			event: bbv1test.MakeEvent(
				&info.Event{
					AccountID: "0000",
					Sender:    "NotAllowed",
				}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							ID: 1234,
						},
					},
				},
				filescontents: map[string]string{
					"OWNERS": "---\n approvers:\n  - 1234\n",
				},
			},
			isAllowed: false,
		},
		{
			name:  "disallowed/same nickname different account id",
			event: bbv1test.MakeEvent(&info.Event{Sender: "Bouffon", AccountID: "6666"}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							DisplayName: "Bouffon",
							ID:          7777,
						},
					},
				},
			},
			isAllowed: false,
		},
		{
			name:  "disallowed/not a valid ok-to-test comment",
			event: bbv1test.MakeEvent(&info.Event{Sender: "Bouffon", AccountID: "6666"}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							ID: ownerAccountID,
						},
					},
				},
				activities: []*bbv1.Activity{
					{
						Comment: bbv1.ActivityComment{
							Text: "not a valid\n /ok-to-test",
							Author: bbv1.User{
								ID: ownerAccountID,
							},
						},
					},
				},
			},
			isAllowed: false,
		},
		{
			name: "allowed/ok-to-test on new line",
			event: bbv1test.MakeEvent(&info.Event{
				AccountID: fmt.Sprintf("%d", otherAccountID),
				Sender:    "NotAllowedAtFirst",
			}),
			fields: fields{
				projectMembers: []*bbv1.UserPermission{
					{
						User: bbv1.User{
							ID: ownerAccountID,
						},
					},
				},
				activities: []*bbv1.Activity{
					{
						Comment: bbv1.ActivityComment{
							Text: "this is a valid\n/ok-to-test",
							Author: bbv1.User{
								ID: ownerAccountID,
							},
						},
					},
				},
			},
			isAllowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			bbclient, scmClient, mux, tearDown, tURL := bbv1test.SetupBBDataCenterClient(ctx)
			defer tearDown()
			bbv1test.MuxProjectMemberShip(t, mux, tt.event, tt.fields.projectMembers)
			bbv1test.MuxRepoMemberShip(t, mux, tt.event, tt.fields.repoMembers)
			bbv1test.MuxPullRequestActivities(t, mux, tt.event, tt.fields.pullRequestNumber, tt.fields.activities)
			bbv1test.MuxFiles(t, mux, tt.event, tt.fields.defaultBranchLatestCommit, "", tt.fields.filescontents, false)

			v := &Provider{
				baseURL:                   tURL,
				Client:                    bbclient,
				ScmClient:                 scmClient,
				defaultBranchLatestCommit: tt.fields.defaultBranchLatestCommit,
				pullRequestNumber:         tt.fields.pullRequestNumber,
				projectKey:                tt.event.Organization,
			}

			got, err := v.IsAllowed(ctx, tt.event)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isAllowed, got, "BitbucketDataCenter.IsAllowed() = %v, want %v", got, tt.isAllowed)
		})
	}
}
