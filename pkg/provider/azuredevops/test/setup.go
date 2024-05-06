package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
)

type MockGitClient struct {
	git.Client
	CreateCommitStatusFunc      func(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error)
	GetPullRequestStatusesFunc  func(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error)
	UpdatePullRequestStatusFunc func(ctx context.Context, args git.UpdatePullRequestStatusesArgs) error
	CreatePullRequestStatusFunc func(ctx context.Context, args git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error)
	CreateAnnotatedTagFunc      func(ctx context.Context, args git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error)
	GetItemContentFunc          func(context.Context, git.GetItemContentArgs) (io.ReadCloser, error)
}

// MockCoreClient mocks the core.Client interface
type MockCoreClient struct {
	core.Client
	TeamMembers                              map[string][]string
	GetTeamsFunc                             func(context.Context, core.GetTeamsArgs) (*[]core.WebApiTeam, error)
	GetTeamMembersWithExtendedPropertiesFunc func(context.Context, core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error)
}

func (m *MockGitClient) CreateCommitStatus(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error) {
	return m.CreateCommitStatusFunc(ctx, args)
}

func (m *MockGitClient) GetPullRequestStatuses(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error) {
	return m.GetPullRequestStatusesFunc(ctx, args)
}

func (m *MockGitClient) UpdatePullRequestStatuses(ctx context.Context, args git.UpdatePullRequestStatusesArgs) error {
	return m.UpdatePullRequestStatusFunc(ctx, args)
}

func (m *MockGitClient) CreatePullRequestStatus(ctx context.Context, args git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error) {
	return m.CreatePullRequestStatusFunc(ctx, args)
}

func (m *MockGitClient) CreateAnnotatedTag(ctx context.Context, args git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error) {
	return m.CreateAnnotatedTagFunc(ctx, args)
}

func (m *MockGitClient) GetItemContent(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBuffer(nil)), nil
}

func (m *MockCoreClient) GetTeams(ctx context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
	var teams []core.WebApiTeam
	for teamID := range m.TeamMembers {
		uuidID, _ := uuid.Parse(teamID)
		teams = append(teams, core.WebApiTeam{Id: &uuidID, Name: &teamID})
	}
	return &teams, nil
}

func (m *MockCoreClient) GetTeamMembersWithExtendedProperties(ctx context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
	uuidID, err := uuid.Parse(*args.TeamId)
	if err != nil {
		return nil, err
	}

	members, exists := m.TeamMembers[uuidID.String()]
	if !exists {
		return nil, nil
	}

	var teamMembers []webapi.TeamMember
	for _, memberID := range members {
		identityID := memberID
		teamMembers = append(teamMembers, webapi.TeamMember{
			Identity: &webapi.IdentityRef{Id: &identityID},
		})
	}
	return &teamMembers, nil
}

func Setup() (*MockGitClient, *MockCoreClient, *http.ServeMux, *httptest.Server, func()) {

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	tearDown := func() {
		server.Close()
	}

	mockGitClient := &MockGitClient{
		CreateCommitStatusFunc: func(ctx context.Context, args git.CreateCommitStatusArgs) (*git.GitStatus, error) {
			return &git.GitStatus{}, nil
		},
		GetPullRequestStatusesFunc: func(ctx context.Context, args git.GetPullRequestStatusesArgs) (*[]git.GitPullRequestStatus, error) {
			return &[]git.GitPullRequestStatus{}, nil
		},
		UpdatePullRequestStatusFunc: func(context.Context, git.UpdatePullRequestStatusesArgs) error {
			return nil
		},
		CreatePullRequestStatusFunc: func(context.Context, git.CreatePullRequestStatusArgs) (*git.GitPullRequestStatus, error) {
			return &git.GitPullRequestStatus{}, nil
		},
		CreateAnnotatedTagFunc: func(context.Context, git.CreateAnnotatedTagArgs) (*git.GitAnnotatedTag, error) {
			return &git.GitAnnotatedTag{}, nil
		},
		GetItemContentFunc: func(context.Context, git.GetItemContentArgs) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(nil)), nil
		},
	}
	mockCoreClient := &MockCoreClient{
		GetTeamsFunc: func(ctx context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
			return &[]core.WebApiTeam{}, nil
		},
		GetTeamMembersWithExtendedPropertiesFunc: func(ctx context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
			return &[]webapi.TeamMember{}, nil
		},
	}
	return mockGitClient, mockCoreClient, mux, server, tearDown
}
