package scm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

type Opts struct {
	GitURL             string
	TargetRefName      string
	BaseRefName        string
	WebURL             string
	Log                *zap.SugaredLogger
	CommitTitle        string
	PushForce          bool
	NoCheckOutFromBase bool
}

type FileChange struct {
	FileName   string
	ChangeType string
	NewName    string
	NewContent string
}

// initializeGitClone initializes git's configuration, clones a given repository, and sets up the given branch.
// Returns the repository path, a cleanup function which should be deferred, and an error, if any occurred.
func initializeGit(t *testing.T, opts *Opts) (string, func(), error) {
	tmpdir := fs.NewDir(t, t.Name())
	fixPwd := env.ChangeWorkingDir(t, tmpdir.Path())
	cleanupFunc := func() {
		fixPwd()
		if os.Getenv("TEST_NOCLEANUP") == "" {
			tmpdir.Remove()
		}
	}
	path := tmpdir.Path()

	var err error

	if _, err = git.RunGit(path, "init"); err != nil {
		return "", func() {}, err
	}
	if _, err = git.RunGit(path, "config", "user.name", "OpenShift Pipelines E2E test"); err != nil {
		return "", func() {}, err
	}
	if _, err = git.RunGit(path, "config", "user.email", "e2e-pipeline@redhat.com"); err != nil {
		return "", func() {}, err
	}

	if _, err = git.RunGit(path, "remote", "add", "-f", "origin", opts.GitURL); err != nil {
		return "", func() {}, err
	}

	if _, err = git.RunGit(path, "fetch", "-a", "origin"); err != nil {
		return "", func() {}, err
	}

	if strings.HasPrefix(opts.TargetRefName, "refs/tags") {
		_, err = git.RunGit(path, "reset", "--hard", "origin/"+opts.BaseRefName)
	} else {
		if opts.NoCheckOutFromBase {
			// Create a new branch without the base reference,
			// which can be helpful for testing when you only want to add specific requested files
			_, err = git.RunGit(path, "checkout", "-B", opts.TargetRefName)
		} else {
			// checkout new branch from base branch
			_, err = git.RunGit(path, "checkout", "-B", opts.TargetRefName, "origin/"+opts.BaseRefName)
		}
	}
	return path, cleanupFunc, err
}

// gitPushPullRetry tries to push the files to the repo, if it fails it will try to rebase and push again.
// Returns the sha of the commit pushed.
func gitPushPullRetry(t *testing.T, opts *Opts, path string) string {
	// use a loop to try multiple times in case of error
	var err error
	count := 0
	for {
		pushForce := "--no-force"
		if opts.PushForce {
			pushForce = "-f"
		}
		if _, err = git.RunGit(path, "push", "origin", pushForce, opts.TargetRefName); err == nil {
			opts.Log.Infof("Pushed files to repo %s branch %s", opts.WebURL, opts.TargetRefName)
			// trying to avoid the multiple events at the time of creation we have a sync
			time.Sleep(5 * time.Second)

			// get sha
			sha, err := git.RunGit(path, "rev-parse", "HEAD")
			assert.NilError(t, err)

			return sha
		}
		if strings.Contains(err.Error(), "non-fast-forward") {
			_, err = git.RunGit(path, "fetch", "-a", "origin")
			assert.NilError(t, err)
			_, err := git.RunGit(path, "pull", "--rebase", "origin", opts.TargetRefName)
			assert.NilError(t, err)
			opts.Log.Infof("Rebased against branch %s", opts.TargetRefName)
			continue
		}
		count++
		if count > 5 {
			t.Fatalf("Failed to push files to repo %s branch %s, %+v", opts.WebURL, opts.TargetRefName, err.Error())
		}
		opts.Log.Errorf("Failed to push files to repo %s branch %s, retrying in 5 seconds, err: %v", opts.WebURL, opts.TargetRefName, err)

		time.Sleep(5 * time.Second)
	}
}

func PushFilesToRefGit(t *testing.T, opts *Opts, entries map[string]string) string {
	path, cleanupFunc, err := initializeGit(t, opts)
	defer cleanupFunc()
	assert.NilError(t, err)

	for filename, content := range entries {
		assert.NilError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
		// write content to filename
		assert.NilError(t, os.WriteFile(filename, []byte(content), 0o600))
	}
	_, err = git.RunGit(path, "add", ".")
	assert.NilError(t, err)

	commitTitle := opts.CommitTitle
	if commitTitle == "" {
		commitTitle = "Committing files from test on " + opts.TargetRefName
	}
	_, err = git.RunGit(path, "-c", "commit.gpgsign=false", "commit", "-m", commitTitle)
	assert.NilError(t, err)

	if strings.HasPrefix(opts.TargetRefName, "refs/tags") {
		_, err = git.RunGit(path, "tag", "-f", filepath.Base(opts.TargetRefName))
		assert.NilError(t, err)
	}

	return gitPushPullRetry(t, opts, path)
}

func ChangeFilesRefGit(t *testing.T, opts *Opts, fileChanges []FileChange) {
	path, cleanupFunc, err := initializeGit(t, opts)
	defer cleanupFunc()
	assert.NilError(t, err)

	for _, fileChange := range fileChanges {
		switch fileChange.ChangeType {
		case "rename":
			_, err = git.RunGit(path, "mv", fileChange.FileName, fileChange.NewName)
			assert.NilError(t, err)
		case "delete":
			assert.NilError(t, os.Remove(fileChange.FileName))
		case "modify":
			assert.NilError(t, os.Remove(fileChange.FileName))
			assert.NilError(t, os.WriteFile(fileChange.FileName, []byte(fileChange.NewContent), 0o600))
		}
	}
	_, err = git.RunGit(path, "add", ".")
	assert.NilError(t, err)

	commitTitle := opts.CommitTitle
	if commitTitle == "" {
		commitTitle = "Committing updated files from test on " + opts.TargetRefName
	}
	_, err = git.RunGit(path, "-c", "commit.gpgsign=false", "commit", "-m", commitTitle)
	assert.NilError(t, err)

	if strings.HasPrefix(opts.TargetRefName, "refs/tags") {
		_, err = git.RunGit(path, "tag", "-f", filepath.Base(opts.TargetRefName))
		assert.NilError(t, err)
	}
	gitPushPullRetry(t, opts, path)
}

// MakeGitCloneURL will make a clone url with username and password.
func MakeGitCloneURL(targetURL, userName, password string) (string, error) {
	// parse hostname of giteaURL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse url %s: %w", targetURL, err)
	}

	return fmt.Sprintf("%s://%s:%s@%s%s", parsedURL.Scheme, userName, password, parsedURL.Host, parsedURL.Path), nil
}
