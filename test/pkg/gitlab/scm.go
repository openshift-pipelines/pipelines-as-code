package gitlab

import (
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

// CreateGitLabProject creates a new GitLab project inside a group and adds
// a webhook pointing to the given hookURL (for example, a smee channel URL
// used to forward events to the controller). The project is initialised with
// a README on the "main" branch.
func CreateGitLabProject(client *gitlab.Client, groupPath, projectName, hookURL, webhookSecret string, logger *zap.SugaredLogger) (*gitlab.Project, error) {
	logger.Infof("Looking up GitLab group %s", groupPath)
	group, _, err := client.Groups.GetGroup(groupPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to look up group %s: %w", groupPath, err)
	}

	logger.Infof("Creating GitLab project %s in group %s (ID %d)", projectName, groupPath, group.ID)
	project, _, err := client.Projects.CreateProject(&gitlab.CreateProjectOptions{
		Name:                 gitlab.Ptr(projectName),
		NamespaceID:          gitlab.Ptr(group.ID),
		InitializeWithReadme: gitlab.Ptr(true),
		DefaultBranch:        gitlab.Ptr("main"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create project %s: %w", projectName, err)
	}
	logger.Infof("Created GitLab project %s (ID %d)", project.PathWithNamespace, project.ID)

	logger.Infof("Adding webhook to project %s pointing to %s", project.PathWithNamespace, hookURL)
	_, _, err = client.Projects.AddProjectHook(project.ID, &gitlab.AddProjectHookOptions{
		URL:                   gitlab.Ptr(hookURL),
		Token:                 gitlab.Ptr(webhookSecret),
		MergeRequestsEvents:   gitlab.Ptr(true),
		NoteEvents:            gitlab.Ptr(true),
		PushEvents:            gitlab.Ptr(true),
		TagPushEvents:         gitlab.Ptr(true),
		EnableSSLVerification: gitlab.Ptr(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add webhook to project %s: %w", project.PathWithNamespace, err)
	}

	return project, nil
}
