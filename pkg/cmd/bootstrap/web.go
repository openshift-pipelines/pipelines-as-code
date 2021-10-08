package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

// openWebBrowser opens the specified URL in the default browser of the user.
func openWebBrowser(url string) error {
	var cmd string
	var args []string

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

// startWebServer starts a webserver that will redirect the user to the github app creation page.
func startWebServer(ctx context.Context, opts *bootstrapOpts, run *params.Run, jeez string) error {
	m := http.NewServeMux()
	s := http.Server{Addr: fmt.Sprintf(":%d", opts.webserverPort), Handler: m}
	codeCh := make(chan string)
	m.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			fmt.Fprint(rw, successTmpl)
			codeCh <- code
		} else {
			fmt.Fprintf(rw, indexTmpl, opts.GithubAPIURL, jeez)
		}
	})
	go func() {
		url := fmt.Sprintf("http://localhost:%d", opts.webserverPort)
		// nolint:forbidigo
		fmt.Printf("🌍 Starting a web browser on %s, click on the button to create your GitHub APP\n", url)
		// nolint:errcheck
		go openWebBrowser(url)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	code := <-codeCh

	err := s.Shutdown(ctx)
	if err != nil {
		return err
	}

	gvcs, err := getGHClient(opts)
	if err != nil {
		return err
	}

	manifest, _, err := gvcs.Apps.CompleteAppManifest(ctx, code)
	if err != nil {
		return err
	}

	err = createPacSecret(ctx, run, opts, manifest)
	if err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🚀 You can now add your newly created application on your repository by going to this URL:\n%s\n", *manifest.HTMLURL)

	// nolint:forbidigo
	fmt.Println("💡 Don't forget to run the \"tkn pac repo create\" to create a new Repository CRD on your cluster.")

	return nil
}
