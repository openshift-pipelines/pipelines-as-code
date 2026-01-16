package gitlab

import (
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func CreateMR(client *gitlab.Client, pid int, sourceBranch, targetBranch, title string) (int, error) {
	mr, _, err := client.MergeRequests.CreateMergeRequest(pid, &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
	})
	if err != nil {
		return -1, err
	}
	return int(mr.IID), nil
}

func CreateTag(client *gitlab.Client, pid int, tagName string) error {
	_, resp, err := client.Tags.CreateTag(pid, &gitlab.CreateTagOptions{
		TagName: gitlab.Ptr(tagName),
		Ref:     gitlab.Ptr("main"),
	})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create tag : %d", resp.StatusCode)
	}

	return nil
}

func DeleteTag(client *gitlab.Client, pid int, tagName string) error {
	resp, err := client.Tags.DeleteTag(pid, tagName)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete tag : %d", resp.StatusCode)
	}

	return nil
}
