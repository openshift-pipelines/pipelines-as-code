package bootstrap

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/google/go-github/v43/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
)

const (
	pacGHRepoOwner = "openshift-pipelines"
	pacGHRepoName  = "pipelines-as-code"
	rawGHURL       = "https://raw.githubusercontent.com"

	openshiftReleaseYaml = "release.yaml"
	k8ReleaseYaml        = "release.k8s.yaml"
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

func installPac(ctx context.Context, run *params.Run, opts *bootstrapOpts) error {
	var latestVersion, latestReleaseYaml, nightlyReleaseYaml string
	k8Ext := ""

	routeExist, _ := checkOpenshiftRoute(run)
	if routeExist {
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
	return nil
}
