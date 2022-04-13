package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
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
			url := opts.GithubAPIURL
			if opts.GithubOrganizationName != "" {
				url = filepath.Join(opts.GithubAPIURL, "organizations", opts.GithubOrganizationName)
			}
			fmt.Fprintf(rw, indexTmpl, url, jeez)
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

	gprovider, err := getGHClient(opts)
	if err != nil {
		return err
	}

	manifest, _, err := gprovider.Apps.CompleteAppManifest(ctx, code)
	if err != nil {
		return err
	}

	err = createPacSecret(ctx, run, opts, manifest)
	if err != nil {
		return err
	}

	if err := info.UpdateInfoConfigMap(ctx, run, &info.Options{
		TargetNamespace: opts.targetNamespace,
		ControllerURL:   opts.RouteName,
		Provider:        provider.ProviderGitHubApp,
	}); err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("🚀 You can now add your newly created application on your repository by going to this URL:\n\n%s\n\n", *manifest.HTMLURL)

	// nolint:forbidigo
	fmt.Println("💡 Don't forget to run the \"tkn pac repo create\" to create a new Repository CRD on your cluster.")

	detectString := detectSelfSignedCertificate(ctx, opts.RouteName)
	if detectString != "" {
		// nolint:forbidigo
		fmt.Println(detectString)
	}

	return nil
}
