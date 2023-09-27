package git

import (
	"os"
	"os/exec"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

func TestGetGitInfo(t *testing.T) {
	// this fails on workflow action due of old git version?
	if os.Getenv("TEST_SKIP_GIT") != "" {
		t.Skip("Skipping Git test")
		return
	}
	gitPath, _ := exec.LookPath("git")
	if gitPath == "" {
		t.Skip("could not find the git binary in path, skipping test")
		return
	}

	tests := []struct {
		name         string
		want         Info
		gitURL       string
		remoteTarget string
		branchName   string
	}{
		{
			name:         "Get git info",
			gitURL:       "https://github.com/chmouel/demo",
			remoteTarget: "origin",
			want: Info{
				URL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Get git info remove .git suffix",
			gitURL:       "git@github.com:chmouel/demo.git",
			remoteTarget: "origin",
			want: Info{
				URL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Transform SSH info",
			gitURL:       "git@github.com:chmouel/demo",
			remoteTarget: "origin",
			want: Info{
				URL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Transform SSH info from upstream",
			gitURL:       "git@github.com:chmouel/demo",
			remoteTarget: "upstream",
			want: Info{
				URL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Get head ref",
			gitURL:       "git@github.com:chmouel/demo",
			remoteTarget: "upstream",
			want: Info{
				Branch: "targetheadbranch",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create temporary file
			tmpFile := fs.NewFile(t, "gitconfig-")
			defer tmpFile.Remove()
			defer env.PatchAll(t, map[string]string{
				"HOME": tmpFile.Path(),
				"PATH": "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
			})()

			newdir := fs.NewDir(t, "TestGetGitInfo")
			defer newdir.Remove()
			gitDir := newdir.Path()
			var err error

			_, err = RunGit(gitDir, "init")
			assert.NilError(t, err)

			_, err = RunGit(gitDir, "config", "--local", "user.email", "foo@foo.com")
			assert.NilError(t, err)

			_, err = RunGit(gitDir, "config", "--local", "user.name", "Mister Ze Foo")
			assert.NilError(t, err)

			_, err = RunGit(gitDir, "remote", "add", tt.remoteTarget, tt.gitURL)
			assert.NilError(t, err)
			_, err = RunGit(gitDir, "commit", "--allow-empty", "-m", "Empty Commit")
			assert.NilError(t, err)
			if tt.want.Branch != "" {
				_, _ = RunGit(gitDir, "checkout", "-b", tt.want.Branch)
			}
			gitinfo := GetGitInfo(gitDir)
			if tt.want.URL != "" {
				assert.Equal(t, gitinfo.URL, tt.want.URL)
			}
			if tt.want.Branch != "" {
				assert.Equal(t, gitinfo.Branch, tt.want.Branch)
			}
		})
	}
}
