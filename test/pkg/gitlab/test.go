package gitlab

import (
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
