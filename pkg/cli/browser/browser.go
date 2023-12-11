package browser

import (
	"os/exec"
	"runtime"
)

// OpenWebBrowser opens the specified URL in the default browser of the user.
func OpenWebBrowser(url string) error {
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
	return exec.Command(cmd, args...).Start()
}
