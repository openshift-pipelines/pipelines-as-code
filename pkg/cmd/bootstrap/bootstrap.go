package bootstrap

import (
	"context"
	"fmt"
	"strings"

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
	secretName     = "github-app-secret"
	defaultVCSType = "github-app"
)

var vcsTargets = []string{"github-app", "github-enteprise-app"}

type bootstrapOpts struct {
	vcsType         string
	installNightly  bool
	skipInstall     bool
	skipGithubAPP   bool
	webserverPort   int
	cliOpts         *params.PacCliOpts
	ioStreams       *ui.IOStreams
	targetNamespace string
	recreateSecret  bool

	RouteName             string
	GithubAPIURL          string
	GithubApplicationName string
	GithubApplicationURL  string
}

const indexTmpl = `
<html>
<body>
  <form method="post" action="%s/settings/apps/new">
  <input type="submit" value="Create your Github APP"></input>
  <input type="hidden" name="manifest" value='%s'"/>
  </form>
</body>
</html>
`

const successTmpl = `
<html><body>You have <span style=\"color: green\">successfully</span> created a new Github application, go back to the tkn pac cli to finish the installation.</body></html>
`

func install(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	// nolint:forbidigo
	fmt.Println("🏃 Checking if Pipelines as Code is installed.")
	if checkNS(ctx, run, opts) {
		// nolint:forbidigo
		fmt.Println("👌 Pipelines as Code is installed.")
	} else if err := installPac(ctx, opts); err != nil {
		return err
	}
	return nil
}

func createSecret(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	var err error
	opts.recreateSecret = checkSecret(ctx, run, opts)
	opts.RouteName, err = detectOpenShiftRoute(ctx, run, opts)
	if err != nil {
		return err
	}

	if err := askQuestions(opts); err != nil {
		return err
	}

	if opts.recreateSecret {
		if err := deleteSecret(ctx, run, opts); err != nil {
			return err
		}
	}

	jeez, err := generateManifest(opts)
	if err != nil {
		return err
	}

	return startWebServer(ctx, opts, run, string(jeez))
}

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

			if err := run.Clients.NewClients(&run.Info); err != nil {
				return err
			}

			if !opts.skipInstall {
				if err := install(ctx, run, opts); err != nil {
					return err
				}
			}

			if !opts.skipGithubAPP {
				if err := createSecret(ctx, run, opts); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&opts.GithubApplicationName, "github-application-name", "", "Github Application Name")
	cmd.PersistentFlags().StringVar(&opts.GithubApplicationURL, "github-application-url", "", "Github Application URL")
	cmd.PersistentFlags().StringVar(&opts.RouteName, "route-url", "", "the URL for the eventlistenner")
	cmd.PersistentFlags().BoolP("no-color", "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().BoolVar(&opts.installNightly, "nightly", false, "Wether to install the nightly Pipelines as Code")
	cmd.PersistentFlags().IntVar(&opts.webserverPort, "webserver-port", 8080, "webserver-port")
	cmd.PersistentFlags().StringVarP(&opts.targetNamespace, "namespace", "n", pacNS, "target namespace where pac is installed")

	cmd.PersistentFlags().BoolVar(&opts.skipInstall, "skip-install", false, "skip Pipelines as Code installation")
	cmd.PersistentFlags().BoolVar(&opts.skipGithubAPP, "skip-github-app", false, "skip creating github application")

	cmd.PersistentFlags().StringVarP(&opts.vcsType, "install-type", "t", defaultVCSType,
		fmt.Sprintf("target install type, choices are: %s ", strings.Join(vcsTargets, ", ")))
	return cmd
}
