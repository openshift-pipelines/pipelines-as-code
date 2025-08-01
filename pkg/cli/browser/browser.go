package browser

import (
	"context"
	"os/exec"
	"runtime"
)

// OpenWebBrowser opens the specified URL in the default browser of the user.
func OpenWebBrowser(ctx context.Context, url string) error {
	var cmd string

	args := []string{}
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}

	args = append(args, url)
	return exec.CommandContext(ctx, cmd, args...).Start()
}
