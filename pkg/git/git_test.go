package git

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func TestGetGitInfo(t *testing.T) {
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
			nd := fs.NewDir(t, "TestGetGitInfo")
			defer nd.Remove()
			gitDir := nd.Path()
			_, _ = RunGit(gitDir, "init")
			_, _ = RunGit(gitDir, "remote", "add", tt.remoteTarget, tt.gitURL)
			_, _ = RunGit(gitDir, "config", "user.email", "foo@foo.com")
			_, _ = RunGit(gitDir, "config", "user.name", "Foo Bar")
			_, _ = RunGit(gitDir, "commit", "--allow-empty", "-m", "Empty Commmit")
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
