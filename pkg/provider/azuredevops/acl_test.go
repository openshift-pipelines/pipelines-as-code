package azuredevops

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops/test"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestIsAllowedAzureDevOps(t *testing.T) {

	type fields struct {
		teamMembers map[string][]string
	}

	tests := []struct {
		name          string
		event         *info.Event
		fields        fields
		isAllowed     bool
		wantErrSubstr string
	}{
		{
			name: "allowed/user is team member",
			event: &info.Event{
				Sender:    "user123",
				ProjectID: "project1",
			},
			fields: fields{
				teamMembers: map[string][]string{
					"00000000-0000-0000-0000-000000000000": {"user123", "user456"},
				},
			},
			isAllowed: true,
		},
		{
			name: "disallowed/user not a team member",
			event: &info.Event{
				Sender:    "user999",
				ProjectID: "project1",
			},
			fields: fields{
				teamMembers: map[string][]string{
					"00000000-0000-0000-0000-000000000000": {"user123", "user456"},
				},
			},
			isAllowed: false,
		},
	}

	mockGit, mockCore, _, _, tearDown := test.Setup()

	defer tearDown()

	logger := zap.NewExample().Sugar()
	defer logger.Sync()
	ctx, _ := rtesting.SetupFakeContext(t)
	provider := Provider{
		GitClient:  mockGit,
		CoreClient: mockCore,
		Logger:     logger,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCore.TeamMembers = tt.fields.teamMembers
			got, err := provider.IsAllowed(ctx, tt.event)
			if tt.wantErrSubstr != "" {
				assert.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, tt.isAllowed, got, "Provider.IsAllowed() = %v, want %v", got, tt.isAllowed)
		})
	}
}
