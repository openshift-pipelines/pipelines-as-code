package gitlab

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestExtractGitLabInfo(t *testing.T) {
	tests := []struct {
		url      string
		name     string
		expected *gitLabInfo
	}{
		{
			name: "custom host",
			url:  "https://gitlab.chmouel.com/group/subgroup/repo/-/blob/main/README.md?ref_type=heads",
			expected: &gitLabInfo{
				Host:        "gitlab.chmouel.com",
				GroupOrUser: "group/subgroup",
				Repository:  "repo",
				Revision:    "main",
				FilePath:    "README.md",
			},
		},
		{
			name: "org repo",
			url:  "https://gitlab.com/org/repo/-/blob/main/README.md",
			expected: &gitLabInfo{
				Host:        "gitlab.com",
				GroupOrUser: "org",
				Repository:  "repo",
				Revision:    "main",
				FilePath:    "README.md",
			},
		},
		{
			name: "long group and subgroups",
			url:  "https://gitlab.com/gitlab-com/partners/alliance/corp/sandbox/another/foo-foo/-/raw/main/hello.txt?ref_type=heads",
			expected: &gitLabInfo{
				Host:        "gitlab.com",
				GroupOrUser: "gitlab-com/partners/alliance/corp/sandbox/another",
				Repository:  "foo-foo",
				Revision:    "main",
				FilePath:    "hello.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := extractGitLabInfo(tt.url)
			assert.NilError(t, err)
			assert.DeepEqual(t, info, tt.expected)
		})
	}
}
