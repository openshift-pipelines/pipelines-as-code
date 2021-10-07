package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/google/go-github/scrape"
	"github.com/google/go-github/v39/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/ui"
	"github.com/spf13/cobra"
)

const (
	pacNS                  = "pipelines-as-code"
	pacLabel               = "eventlistener=pipelines-as-code-interceptor"
	openShiftRouteGroup    = "route.openshift.io"
	openShiftRouteVersion  = "v1"
	openShiftRouteResource = "routes"
	// nolint: gosec
	secretName = "github-app-secret"
)

type bootstrapOpts struct {
	installNightly  bool
	webserverPort   int
	cliOpts         *params.PacCliOpts
	ioStreams       *ui.IOStreams
	RouteName       string
	ApplicationName string
	ApplicationURL  string
	targetNamespace string
	RecreateSecret  bool
}

const indexTmpl = `
<html>
<body>
  <form method="post" action="https://github.com/settings/apps/new">
  <input type="submit" value="Create your Github APP"></input>
  <input type="hidden" name="manifest" value='%s'"/>
  </form>
</body>
</html>
`

const successTmpl = `
<html><body>You have <span style=\"color: green\">successfully</span> created a new Github application, go back to the tkn pac cli to finish the installation.</body></html>
`

func Command(run *params.Run, ioStreams *ui.IOStreams) *cobra.Command {
	opts := &bootstrapOpts{}
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Long:  "Bootstrap Pipelines as Code",
		Short: "Bootstrap Pipelines as Code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			ctx := context.Background()
			opts.ioStreams = ioStreams
			opts.cliOpts, err = params.NewCliOptions(cmd)
			if err != nil {
				return err
			}
			opts.ioStreams.SetColorEnabled(!opts.cliOpts.NoColoring)

			err = run.Clients.NewClients(&run.Info)
			if err != nil {
				return err
			}

			// nolint:forbidigo
			fmt.Println("🏃 Checking if Pipelines as Code is installed.")
			if checkNS(ctx, run, opts) {
				// nolint:forbidigo
				fmt.Println("👌 Pipelines as Code is installed.")
			} else if err := installPac(ctx, opts); err != nil {
				return err
			}

			opts.RecreateSecret = checkSecret(ctx, run, opts)
			opts.RouteName, err = detectOpenShiftRoute(ctx, run, opts)
			if err != nil {
				return err
			}

			if err := askQuestions(opts); err != nil {
				return err
			}

			if opts.RecreateSecret {
				if err := deleteSecret(ctx, run, opts); err != nil {
					return err
				}
			}

			jeez, err := generateManifest(opts)
			if err != nil {
				return err
			}

			return startWebServer(ctx, opts, run, string(jeez))
		},
	}
	cmd.PersistentFlags().StringVar(&opts.ApplicationName, "application-name", "", "Application Name")
	cmd.PersistentFlags().StringVar(&opts.ApplicationURL, "application-url", "", "Application URL")
	cmd.PersistentFlags().StringVar(&opts.RouteName, "route-url", "", "The URL for the eventlistenner")
	cmd.PersistentFlags().BoolP("no-color", "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().BoolVar(&opts.installNightly, "nightly", false, "Wether to install the nightly Pipelines as Code")
	cmd.PersistentFlags().IntVar(&opts.webserverPort, "webserver-port", 8080, "webserver-port")
	cmd.PersistentFlags().StringVarP(&opts.targetNamespace, "namespace", "n", pacNS, "target namespace where pac is installed")
	return cmd
}

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
			fmt.Fprintf(rw, indexTmpl, jeez)
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
	gvcs := github.NewClient(nil)
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

// generateManifest generate manifest from the given options
func generateManifest(opts *bootstrapOpts) ([]byte, error) {
	sc := scrape.AppManifest{
		Name:           github.String(opts.ApplicationName),
		URL:            github.String(opts.ApplicationURL),
		HookAttributes: map[string]string{"url": opts.RouteName},
		RedirectURL:    github.String(fmt.Sprintf("http://localhost:%d", opts.webserverPort)),
		Description:    github.String("Pipilines as Code Application"),
		Public:         github.Bool(true),
		DefaultEvents: []string{
			"commit_comment",
			"issue_comment",
			"pull_request",
			"pull_request_review",
			"pull_request_review_comment",
			"push",
		},
		DefaultPermissions: &github.InstallationPermissions{
			Checks:           github.String("write"),
			Contents:         github.String("write"),
			Issues:           github.String("write"),
			Members:          github.String("read"),
			Metadata:         github.String("read"),
			OrganizationPlan: github.String("read"),
			PullRequests:     github.String("write"),
		},
	}
	return json.Marshal(sc)
}
