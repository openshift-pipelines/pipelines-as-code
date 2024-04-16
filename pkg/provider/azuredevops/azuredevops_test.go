package azuredevops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	statusOpts "github.com/openshift-pipelines/pipelines-as-code/pkg/provider"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

type MockGitClient struct {
	git.Client              // Embedding git.Client for future-proofing against new methods
	createCommitStatus      func(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error)
	getPullRequestStatuses  func(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error)
	updatePullRequestStatus func(ctx context.Context, args git.UpdatePullRequestStatusesArgs) error
	createPullRequestStatus func(ctx context.Context, args git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error)
	createAnnotatedTag      func(ctx context.Context, args git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error)
}

func (m *MockGitClient) CreateCommitStatus(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error) {
	return m.createCommitStatus(ctx, args)
}

func (m *MockGitClient) GetPullRequestStatuses(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error) {
	return m.getPullRequestStatuses(ctx, args)
}

func (m *MockGitClient) UpdatePullRequestStatuses(ctx context.Context, args git.UpdatePullRequestStatusesArgs) error {
	return m.updatePullRequestStatus(ctx, args)
}

func (m *MockGitClient) CreatePullRequestStatus(ctx context.Context, args git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error) {
	return m.createPullRequestStatus(ctx, args)
}

func (m *MockGitClient) CreateAnnotatedTag(ctx context.Context, args git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error) {
	if m.createAnnotatedTag != nil {
		return m.createAnnotatedTag(ctx, args)
	}
	return nil, nil
}

func Setup(t *testing.T) (*MockGitClient, *http.ServeMux, func()) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	tearDown := func() {
		server.Close()
	}

	mockClient := &MockGitClient{
		createCommitStatus: func(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error) {
			return &git.GitStatus{}, nil
		},
		getPullRequestStatuses: func(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error) {
			return &[]git.GitPullRequestStatus{}, nil
		},
		updatePullRequestStatus: func(ctx context.Context, args git.UpdatePullRequestStatusesArgs) error {
			return nil
		},
		createPullRequestStatus: func(ctx context.Context, args git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error) {
			return &git.GitPullRequestStatus{}, nil
		},
		createAnnotatedTag: func(ctx context.Context, args git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error) {
			return &git.GitAnnotatedTag{}, nil
		},
	}

	return mockClient, mux, tearDown
}

func getProvider(mockClient *MockGitClient) *Provider {
	provider := &Provider{
		Client: mockClient,
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
	return provider
}

func TestCreateStatus(t *testing.T) {
	mockClient, _, tearDown := Setup(t)
	defer tearDown()

	ctx := context.Background()

	provider := getProvider(mockClient)

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
