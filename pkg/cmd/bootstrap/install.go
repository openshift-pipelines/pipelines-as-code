package bootstrap

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/google/go-github/v39/github"
)

var (
	pacGHRepoOwner = "openshift-pipelines"
	pacGHRepoName  = "pipelines-as-code"
	rawGHURL       = "https://raw.githubusercontent.com"
)

func getLatestRelease(ctx context.Context) (string, string, error) {
	// Always go to public
	gh := github.NewClient(nil)
	release, _, err := gh.Repositories.GetLatestRelease(ctx, pacGHRepoOwner, pacGHRepoName)
	if err != nil {
		return "", "", err
	}
	return release.GetTagName(),
		fmt.Sprintf("%s/%s/%s/release-%s/release-%s.yaml",
			rawGHURL, pacGHRepoOwner, pacGHRepoName, release.GetTagName(), release.GetTagName()),
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

func installPac(ctx context.Context, opts *bootstrapOpts) error {
	var latestversion, latestReleaseYaml string

	if opts.installNightly {
		latestReleaseYaml = fmt.Sprintf("%s/%s/%s/nightly/release.yaml",
			rawGHURL, pacGHRepoOwner, pacGHRepoName)
		latestversion = "nightly"
	} else {
		var err error
		latestversion, latestReleaseYaml, err = getLatestRelease(ctx)
		if err != nil {
			return err
		}
	}

	doinstall, err := askYN(opts, true, "🕵️ Pipelines as Code doesn't seem installed", fmt.Sprintf("Do you want me to install Pipelines as Code %s?", latestversion))
	if err != nil {
		return err
	}
	if !doinstall {
		return fmt.Errorf("i will let you install Pipelines as Code")
	}

	if err := kubectlApply(latestReleaseYaml); err != nil {
		return err
	}

	// nolint:forbidigo
	fmt.Printf("✓ Pipelines-as-Code %s has been installed\n", latestversion)
	return nil
}
