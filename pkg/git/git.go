package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Info struct {
	TargetURL    string
	TopLevelPath string
}

func RunGit(dir string, args ...string) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		// nolint: nilerr
		return "", nil
	}
	c := exec.Command(gitPath, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	// This is the optional working directory. If not set, it defaults to the current
	// working directory of the process.
	if dir != "" {
		c.Dir = dir
	}
	if err := c.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
}

// GetGitInfo try to detect the current remote for this URL return the origin url transformed and the topdir
func GetGitInfo(dir string) Info {
	gitURL, err := RunGit(dir, "remote", "get-url", "origin")
	if err != nil {
		gitURL, err = RunGit(dir, "remote", "get-url", "upstream")
		if err != nil {
			return Info{}
		}
	}
	gitURL = strings.TrimSpace(gitURL)
	gitURL = strings.TrimSuffix(gitURL, ".git")

	if strings.HasPrefix(gitURL, "git@") {
		sp := strings.Split(gitURL, ":")
		prefix := strings.ReplaceAll(sp[0], "git@", "https://")
		gitURL = fmt.Sprintf("%s/%s", prefix, strings.Join(sp[1:], ":"))
	}

	brootdir, err := RunGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Info{}
	}

	return Info{
		TargetURL:    gitURL,
		TopLevelPath: strings.TrimSpace(brootdir),
	}
}
