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
		fmt.Sprintf("%s/%s/%s/release-%s/release-%s%s.yaml",
			rawGHURL, pacGHRepoOwner, pacGHRepoName, release.GetTagName(), release.GetTagName(), k8release),
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
	var latestversion, latestReleaseYaml, nightlyReleaseYaml string
	k8Ext := ""

	routeURL, _ := detectOpenShiftRoute(ctx, run, opts)
	if routeURL != "" {
		nightlyReleaseYaml = openshiftReleaseYaml
	} else {
		nightlyReleaseYaml = k8ReleaseYaml
		k8Ext = ".k8s"
	}

	if opts.installNightly {
		latestReleaseYaml = fmt.Sprintf("%s/%s/%s/nightly/%s",
			rawGHURL, pacGHRepoOwner, pacGHRepoName, nightlyReleaseYaml)
		latestversion = "nightly"
	} else {
		var err error
		latestversion, latestReleaseYaml, err = getLatestRelease(ctx, k8Ext)
		if err != nil {
			return err
		}
	}

	if !opts.forceInstall {
		doinstall, err := askYN(true,
			"🕵️ Pipelines as Code doesn't seems to be installed",
			fmt.Sprintf("Do you want me to install Pipelines as Code %s?", latestversion))
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

	// nolint:forbidigo
	fmt.Printf("✓ Pipelines-as-Code %s has been installed\n", latestversion)
	return nil
}
