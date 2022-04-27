package gitlab

import (
	ghlib "github.com/xanzy/go-gitlab"
)

var (
	commitAuthor = "OpenShift Pipelines E2E test"
	commitEmail  = "e2e-pipelines@redhat.com"
)

func PushFilesToRef(client *ghlib.Client, commitMessage, baseBranch, targetRef string, pid int, files map[string]string, targetFile string) error {
	fullYaml := ""
	for _, content := range files {
		fullYaml += "---\n"
		fullYaml += content
	}
	_, _, err := client.RepositoryFiles.CreateFile(pid, targetFile, &ghlib.CreateFileOptions{
		Branch:        ghlib.String(targetRef),
		StartBranch:   ghlib.String(baseBranch),
		AuthorEmail:   ghlib.String(commitEmail),
		AuthorName:    ghlib.String(commitAuthor),
		Content:       ghlib.String(fullYaml),
		CommitMessage: ghlib.String(commitMessage),
	})
	if err != nil {
		return err
	}
	return nil
}

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
