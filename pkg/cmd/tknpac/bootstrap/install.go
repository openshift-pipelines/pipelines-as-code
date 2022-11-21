package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v48/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/random"

	_ "embed"
)

//go:embed templates/gosmee.yaml
var gosmeeYaml string

const (
	pacGHRepoOwner = "openshift-pipelines"
	pacGHRepoName  = "pipelines-as-code"
	rawGHURL       = "https://raw.githubusercontent.com"

	openshiftReleaseYaml = "release.yaml"
	k8ReleaseYaml        = "release.k8s.yaml"

	gosmeeInstallHelpText = `Pipelines as Code does not install a Ingress object to make the controller accessing from the internet
we can install a webhook forwarder called gosmee (https://github.com/chmouel/gosmee) using a %s URL
this will let your git platform provider (ie: Github) to reach the controller without having to be having public access`
	minNumOfCharForRandomForwarderID = 16
)

func getLatestRelease(ctx context.Context, k8release string) (string, string, error) {
	// Always go to public
	gh := github.NewClient(nil)
	release, _, err := gh.Repositories.GetLatestRelease(ctx, pacGHRepoOwner, pacGHRepoName)
	if err != nil {
		return "", "", err
	}
	return release.GetTagName(),
		fmt.Sprintf("%s/%s/%s/release-%s/release%s.yaml",
			rawGHURL, pacGHRepoOwner, pacGHRepoName, release.GetTagName(), k8release),
		nil
}

// kubectlApply get kubectl binary and apply a yaml file.
func kubectlApply(uri string) error {
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "apply", "-f", uri)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

func installGosmeeForwarder(opts *bootstrapOpts) error {
	gosmeInstall, err := askYN(true, fmt.Sprintf(gosmeeInstallHelpText, opts.forwarderURL), "Do you want me to install the gosmee forwarder?", opts.ioStreams.Out)
	if err != nil {
		return err
	}
	if !gosmeInstall {
		return fmt.Errorf("please install a ingress object pointing to the controller service as documented here: https://is.gd/FzI0eb and pass the full ingress url as argument to the --route-url flag")
	}

	// maybe we can use https://webhook.chmouel.com too
	opts.RouteName = fmt.Sprintf("https://smee.io/%s", random.AlphaString(minNumOfCharForRandomForwarderID))
	tmpl := strings.ReplaceAll(gosmeeYaml, "FORWARD_URL", opts.RouteName)
	f, err := os.CreateTemp("", "pac-gosmee")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(tmpl); err != nil {
		return err
	}
	if err := kubectlApply(f.Name()); err != nil {
		return err
	}
	fmt.Fprintf(opts.ioStreams.Out, "💡 Your gosmee forward URL is %s\n", opts.RouteName)
	return nil
}

func installPac(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	var latestVersion, latestReleaseYaml, nightlyReleaseYaml string
	k8Ext := ""

	isOpenShift, _ := checkOpenshiftRoute(run)
	if isOpenShift {
		nightlyReleaseYaml = openshiftReleaseYaml
	} else {
		nightlyReleaseYaml = k8ReleaseYaml
		k8Ext = ".k8s"
	}

	if opts.installNightly {
		latestReleaseYaml = fmt.Sprintf("%s/%s/%s/nightly/%s",
			rawGHURL, pacGHRepoOwner, pacGHRepoName, nightlyReleaseYaml)
		latestVersion = "nightly"
	} else {
		var err error
		latestVersion, latestReleaseYaml, err = getLatestRelease(ctx, k8Ext)
		if err != nil {
			return err
		}
	}

	if !opts.forceInstall {
		doinstall, err := askYN(true,
			fmt.Sprintf("🕵️ Pipelines as Code doesn't seems to be installed in %s namespace", opts.targetNamespace),
			fmt.Sprintf("Do you want me to install Pipelines as Code %s?", latestVersion), opts.ioStreams.Out)
		if err != nil {
			return err
		}
		if !doinstall {
			return fmt.Errorf("i will let you install Pipelines as Code")
		}
	}

	if err := kubectlApply(latestReleaseYaml); err != nil {
		return err
	}

	fmt.Fprintf(opts.ioStreams.Out, "✓ Pipelines-as-Code %s has been installed\n", latestVersion)

	if !isOpenShift && opts.RouteName == "" {
		return installGosmeeForwarder(opts)
	}
	return nil
}
