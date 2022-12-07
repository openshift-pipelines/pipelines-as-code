package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/spf13/cobra"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pacNS                  = "pipelines-as-code"
	openShiftRouteGroup    = "route.openshift.io"
	openShiftRouteVersion  = "v1"
	openShiftRouteResource = "routes"
	secretName             = "pipelines-as-code-secret"
	defaultProviderType    = "github-app"
	// https://webhook.chmouel.com/ is a good value too :p
	defaultWebForwarderURL = "https://smee.io"
)

var providerTargets = []string{"github-app", "github-enterprise-app"}

type bootstrapOpts struct {
	providerType      string
	installNightly    bool
	skipInstall       bool
	skipGithubAPP     bool
	forceInstall      bool
	webserverPort     int
	cliOpts           *cli.PacCliOpts
	ioStreams         *cli.IOStreams
	targetNamespace   string
	autoDetectedRoute bool
	forwarderURL      string
	dashboardURL      string

	RouteName              string
	GithubAPIURL           string
	GithubApplicationName  string
	GithubApplicationURL   string
	GithubOrganizationName string
	forceGitHubApp         bool
}

const infoConfigMap = "pipelines-as-code-info"

const indexTmpl = `
<html>
<body>
  <form method="post" action="%s/settings/apps/new">
  <input type="submit" value="Create your GitHub APP"></input>
  <input type="hidden" name="manifest" value='%s'"/>
  </form>
</body>
</html>
`

const successTmpl = `
<html><body>You have <span style=\"color: green\">successfully</span> created a new GitHub application, go back to the tkn pac cli to finish the installation.</body></html>
`

func install(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	if !opts.forceInstall {
		fmt.Fprintln(opts.ioStreams.Out, "🏃 Checking if Pipelines as Code is installed.")
	}
	tektonInstalled, err := checkPipelinesInstalled(run)
	if err != nil {
		return err
	}
	if !tektonInstalled {
		return fmt.Errorf("a Tekton installation has not been found on this cluster, install Tekton first before launching this command")
	}

	// if we gt a ns back it means it has been detected in here so keep it as is.
	// or else just set the default to pacNS
	installed, ns, err := DetectPacInstallation(ctx, opts.targetNamespace, run)

	// installed but there is error for missing resources
	if installed && err != nil && !opts.forceInstall {
		return err
	}
	if ns != "" {
		opts.targetNamespace = ns
	} else if opts.targetNamespace == "" {
		opts.targetNamespace = pacNS
	}

	if !opts.forceInstall && err == nil && installed {
		fmt.Fprintln(opts.ioStreams.Out, "👌 Pipelines as Code is already installed.")
	} else if err := installPac(ctx, run, opts); err != nil {
		return err
	}
	return nil
}

func createSecret(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	var err error

	if opts.RouteName == "" {
		opts.RouteName, _ = DetectOpenShiftRoute(ctx, run, opts.targetNamespace)
		if opts.RouteName != "" {
			opts.autoDetectedRoute = true
		}
	}
	if err := askQuestions(opts); err != nil {
		return err
	}

	if opts.forceGitHubApp {
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

func Command(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	opts := &bootstrapOpts{
		ioStreams: ioStreams,
	}
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Long:  "Bootstrap Pipelines as Code",
		Short: "Bootstrap Pipelines as Code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			opts.cliOpts = cli.NewCliOptions(cmd)
			opts.ioStreams.SetColorEnabled(!opts.cliOpts.NoColoring)
			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}

			if !opts.skipInstall {
				if err := install(ctx, run, opts); err != nil {
					return err
				}
			}

			pacInfo, err := info.GetPACInfo(ctx, run, opts.targetNamespace)
			if err != nil {
				return err
			}

			if !opts.forceGitHubApp {
				if pacInfo.Provider == provider.ProviderGitHubApp {
					fmt.Fprintln(opts.ioStreams.Out, "👌 Skips bootstrapping GitHub App, as one is already configured. Please pass --force-configure to override existing")
					return nil
				}
			}

			if !opts.skipGithubAPP {
				if err := createSecret(ctx, run, opts); err != nil {
					return err
				}
			}
			return nil
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.AddCommand(GithubApp(run, ioStreams))

	addCommonFlags(cmd, ioStreams)
	addGithubAppFlag(cmd, opts)

	cmd.PersistentFlags().BoolVar(&opts.forceInstall, "force-install", false, "whether we should force pac install even if it's already installed")
	cmd.PersistentFlags().BoolVar(&opts.skipInstall, "skip-install", false, "skip Pipelines as Code installation")
	cmd.PersistentFlags().BoolVar(&opts.skipGithubAPP, "skip-github-app", false, "skip creating github application")

	return cmd
}

func GithubApp(run *params.Run, ioStreams *cli.IOStreams) *cobra.Command {
	opts := &bootstrapOpts{
		ioStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:   "github-app",
		Long:  "A command helper to help you create the Pipelines as Code GitHub Application",
		Short: "Create PAC GitHub Application",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			opts.cliOpts = cli.NewCliOptions(cmd)
			opts.ioStreams.SetColorEnabled(!opts.cliOpts.NoColoring)
			if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
				return err
			}

			var err error
			var installed bool
			installed, opts.targetNamespace, err = DetectPacInstallation(ctx, opts.targetNamespace, run)
			if err != nil {
				return err
			}
			// installed but there is error for missing resources
			if installed && err != nil && !opts.forceInstall {
				return err
			}

			pacInfo, err := info.GetPACInfo(ctx, run, opts.targetNamespace)
			if err != nil {
				return err
			}

			if !opts.forceGitHubApp {
				if pacInfo.Provider == provider.ProviderGitHubApp {
					fmt.Fprintln(opts.ioStreams.Out, "👌 Skips bootstrapping GitHub App, as one is already configured. Please pass --force-configure to override existing")
					return nil
				}
			}

			if b, _ := askYN(false, "", "Are you using GitHub Enterprise?", opts.ioStreams.Out); b {
				opts.providerType = "github-enterprise-app"
			}

			return createSecret(ctx, run, opts)
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	addCommonFlags(cmd, ioStreams)
	addGithubAppFlag(cmd, opts)

	cmd.PersistentFlags().StringVarP(&opts.targetNamespace, "namespace", "n", "", "target namespace where pac is installed")
	return cmd
}

func DetectPacInstallation(ctx context.Context, wantedNS string, run *params.Run) (bool, string, error) {
	var installed bool
	_, err := run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories("").List(ctx, metav1.ListOptions{})
	if err != nil && kapierror.IsNotFound(err) {
		return false, "", nil
	}

	installed = true
	if wantedNS != "" {
		_, err := run.Clients.Kube.CoreV1().ConfigMaps(wantedNS).Get(ctx, infoConfigMap, metav1.GetOptions{})
		if err == nil {
			return installed, wantedNS, nil
		}
		return installed, "", fmt.Errorf("could not detect Pipelines as Code configmap in %s namespace : %w, please reinstall", wantedNS, err)
	}

	cms, err := run.Clients.Kube.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{
		LabelSelector: configMapPacLabel,
	})
	if err == nil {
		for _, cm := range cms.Items {
			if cm.Name == infoConfigMap {
				return installed, cm.Namespace, nil
			}
		}
	}
	return installed, "", fmt.Errorf("could not detect Pipelines as Code configmap on the cluster, please reinstall")
}

func addGithubAppFlag(cmd *cobra.Command, opts *bootstrapOpts) {
	cmd.PersistentFlags().StringVar(&opts.GithubOrganizationName, "github-organization-name", "", "Whether you want to target an organization instead of the current user")
	cmd.PersistentFlags().StringVar(&opts.GithubApplicationName, "github-application-name", "", "GitHub Application Name")
	cmd.PersistentFlags().StringVar(&opts.GithubApplicationURL, "github-application-url", "", "GitHub Application URL")
	cmd.PersistentFlags().StringVarP(&opts.GithubAPIURL, "github-api-url", "", "", "Github Enterprise API URL")
	cmd.PersistentFlags().StringVar(&opts.RouteName, "route-url", "", "The public URL for the pipelines-as-code controller")
	cmd.PersistentFlags().StringVar(&opts.forwarderURL, "web-forwarder-url", defaultWebForwarderURL, "the web forwarder url")
	cmd.PersistentFlags().StringVar(&opts.dashboardURL, "dashboard-url", "", "the full URL to the tekton dashboard ")
	cmd.PersistentFlags().BoolVar(&opts.installNightly, "nightly", false, "Whether to install the nightly Pipelines as Code")
	cmd.PersistentFlags().IntVar(&opts.webserverPort, "webserver-port", 8080, "Webserver port")
	cmd.PersistentFlags().StringVarP(&opts.providerType, "install-type", "t", defaultProviderType,
		fmt.Sprintf("target install type, choices are: %s ", strings.Join(providerTargets, ", ")))
	cmd.PersistentFlags().BoolVar(&opts.forceGitHubApp, "force-configure", false, "Whether we should override existing GitHub App")
}

func addCommonFlags(cmd *cobra.Command, ioStreams *cli.IOStreams) {
	cmd.PersistentFlags().BoolP("no-color", "C", !ioStreams.ColorEnabled(), "disable coloring")
}
