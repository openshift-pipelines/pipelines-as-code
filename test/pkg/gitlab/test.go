package gitlab

import (
	"fmt"
	"net/http"

	ghlib "github.com/xanzy/go-gitlab"
)

func CreateMR(client *ghlib.Client, pid int, sourceBranch, targetBranch, title string) (int, error) {
	mr, _, err := client.MergeRequests.CreateMergeRequest(pid, &ghlib.CreateMergeRequestOptions{
		Title:        &title,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
	})
	if err != nil {
		return -1, err
	}
	return mr.IID, nil
}

func CreateTag(client *ghlib.Client, pid int, tagName string) error {
	_, resp, err := client.Tags.CreateTag(pid, &ghlib.CreateTagOptions{
		TagName: ghlib.Ptr(tagName),
		Ref:     ghlib.Ptr("main"),
	})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create tag : %d", resp.StatusCode)
	}

	return nil
}

func DeleteTag(client *ghlib.Client, pid int, tagName string) error {
	resp, err := client.Tags.DeleteTag(pid, tagName)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete tag : %d", resp.StatusCode)
	}

	return nil
}
