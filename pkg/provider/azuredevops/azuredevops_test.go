package azuredevops

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	statusOpts "github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops/test"
	"go.uber.org/zap"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestCreateStatus(t *testing.T) {
	mockGit, mockCore, _, _, tearDown := test.Setup()
	defer tearDown()

	logger := zap.NewExample().Sugar()
	defer logger.Sync()
	ctx, _ := rtesting.SetupFakeContext(t)
	provider := Provider{
		GitClient:  mockGit,
		CoreClient: mockCore,
		Logger:     logger,
		run: &params.Run{
			Info: info.Info{
				Pac: &info.PacOpts{
					Settings: &settings.Settings{
						ApplicationName: "Pipelines as Code CI",
					},
				},
			},
		},
	}

	testCases := []struct {
		name        string
		event       *info.Event
		statusOpts  statusOpts.StatusOpts
		expectError bool
	}{
		{
			name: "Git Push - Success",
			event: &info.Event{
				EventType: "git.push",
			},

			statusOpts: statusOpts.StatusOpts{
				Conclusion: "success",
			},
			expectError: false,
		},
		{
			name: "Git Push - Failed",
			event: &info.Event{
				EventType: "git.push",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "failure",
			},
			expectError: false,
		},
		{
			name: "Git Push - Pending",
			event: &info.Event{
				EventType: "git.push",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "pending",
				Status:     "in_progress",
			},
			expectError: false,
		},
		{
			name: "Git Pull Request - Success",
			event: &info.Event{
				EventType: "git.pullrequest.created",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "success",
			},
			expectError: false,
		},
		{
			name: "Git Pull Request - Pending",
			event: &info.Event{
				EventType: "git.pullrequest.created",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "pending",
			},
			expectError: false,
		},
		{
			name: "Git Push - Unknown status",
			event: &info.Event{
				EventType: "git.push",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "abc",
			},
			expectError: false,
		},
		{
			name: "Non-existent Resource",
			event: &info.Event{
				EventType: "git.push",
			},
			statusOpts: statusOpts.StatusOpts{
				Conclusion: "error",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := provider.CreateStatus(ctx, tc.event, tc.statusOpts)
			if (err != nil) != tc.expectError {
				t.Errorf("Test %s expected error: %v, got: %v", tc.name, tc.expectError, err)
			}
		})
	}
}
