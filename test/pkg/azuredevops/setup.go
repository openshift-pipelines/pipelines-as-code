package azuredevops

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	azprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/provider/azuredevops"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	"gotest.tools/v3/assert"
)

func getProjectNameFromURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	pathSegments := strings.Split(parsedURL.Path, "/")

	if len(pathSegments) > 2 {
		return pathSegments[2], nil
	}
	return "", fmt.Errorf("project name not found in the URL")
}

func Setup(ctx context.Context) (*params.Run, options.E2E, azprovider.Provider, error) {
	adoToken := os.Getenv("TEST_AZURE_DEVOPS_TOKEN")
	adoRepo := os.Getenv("TEST_AZURE_DEVOPS_REPO")

	for _, value := range []string{"AZURE_DEVOPS_TOKEN", "AZURE_DEVOPS_REPO"} {
		if env := os.Getenv("TEST_" + value); env == "" {
			return nil, options.E2E{}, azprovider.Provider{}, fmt.Errorf("\"TEST_%s\" env variable is required, skipping", value)
		}
	}

	adoOrganization, err := azprovider.ExtractBaseURL(adoRepo)
	if err != nil {
		return nil, options.E2E{}, azprovider.Provider{}, err
	}

	adoProject, err := getProjectNameFromURL(adoRepo)
	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return nil, options.E2E{}, azprovider.Provider{}, err
	}

	e2eoptions := options.E2E{
		Organization: adoOrganization,
		Repo:         adoRepo,
		ProjectName:  adoProject,
	}

	adoClient := azprovider.Provider{}
	event := info.NewEvent()
	event.Provider = &info.Provider{
		Token: adoToken,
		URL:   adoRepo,
	}
	event.Organization = adoOrganization

	if err := adoClient.SetClient(ctx, nil, event, nil, nil); err != nil {
		return nil, options.E2E{}, azprovider.Provider{}, err
	}

	return run, e2eoptions, adoClient, nil
}

func TearDown(ctx context.Context, t *testing.T, runcnx *params.Run, azProvider azprovider.Provider, opts options.E2E, prID *int, targetNS string, ref *string, commitID *string) {

	if os.Getenv("TEST_NOCLEANUP") == "true" {
		runcnx.Clients.Log.Infof("Not cleaning up PR since TEST_NOCLEANUP is set")
		return
	}
	runcnx.Clients.Log.Infof("Abandoning PR #%d", prID)

	statusAbandoned := git.PullRequestStatusValues.Abandoned
	prUpdate := git.GitPullRequest{
		Status: &statusAbandoned,
	}

	runcnx.Clients.Log.Infof("Abandoning PR #%d", prID)

	// Update the pull request to abandon it
	_, err := azProvider.Client.UpdatePullRequest(ctx, git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &prUpdate,
		RepositoryId:           &opts.ProjectName,
		PullRequestId:          prID,
		Project:                &opts.ProjectName,
	})
	assert.NilError(t, err)

	//Delete the branch by updating the reference to an empty object ID
	emptyObjectID := "0000000000000000000000000000000000000000" // This is used to indicate a deletion

	refUpdate := git.GitRefUpdate{
		Name:        ref,
		OldObjectId: commitID,
		NewObjectId: &emptyObjectID,
	}

	refUpdates := []git.GitRefUpdate{refUpdate}
	_, err = azProvider.Client.UpdateRefs(ctx, git.UpdateRefsArgs{
		RefUpdates:   &refUpdates,
		RepositoryId: &opts.ProjectName,
		Project:      &opts.ProjectName,
	})
	assert.NilError(t, err)

	repository.NSTearDown(ctx, t, runcnx, targetNS)
}
