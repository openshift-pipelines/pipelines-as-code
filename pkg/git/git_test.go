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
	}{
		{
			name:         "Get git info",
			gitURL:       "https://github.com/chmouel/demo",
			remoteTarget: "origin",
			want: Info{
				TargetURL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Get git info remove .git suffix",
			gitURL:       "git@github.com:chmouel/demo.git",
			remoteTarget: "origin",
			want: Info{
				TargetURL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Transform SSH info",
			gitURL:       "git@github.com:chmouel/demo",
			remoteTarget: "origin",
			want: Info{
				TargetURL: "https://github.com/chmouel/demo",
			},
		},
		{
			name:         "Transform SSH info from upstream",
			gitURL:       "git@github.com:chmouel/demo",
			remoteTarget: "upstream",
			want: Info{
				TargetURL: "https://github.com/chmouel/demo",
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

			assert.Equal(t, GetGitInfo(gitDir).TargetURL, tt.want.TargetURL)
		})
	}
}
