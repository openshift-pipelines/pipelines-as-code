package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/cctx"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/options"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/setup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Setup(ctx context.Context, onSecondController, viaDirectWebhook bool) (context.Context, *params.Run, options.E2E, *github.Provider, error) {
	if err := setup.RequireEnvs(
		"TEST_EL_URL",
		"TEST_GITHUB_API_URL",
		"TEST_GITHUB_TOKEN",
		"TEST_GITHUB_REPO_OWNER_GITHUBAPP",
		"TEST_EL_WEBHOOK_SECRET",
	); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}

	githubToken := ""
	githubURL := os.Getenv("TEST_GITHUB_API_URL")
	githubRepoOwnerGithubApp := os.Getenv("TEST_GITHUB_REPO_OWNER_GITHUBAPP")
	githubRepoOwnerDirectWebhook := os.Getenv("TEST_GITHUB_REPO_OWNER_WEBHOOK")
	// EL_URL mean CONTROLLER URL, it's called el_url because a long time ago pac was based on trigger
	controllerURL := os.Getenv("TEST_EL_URL")

	if onSecondController {
		if err := setup.RequireEnvs(
			"TEST_GITHUB_SECOND_API_URL",
			"TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP",
			"TEST_GITHUB_SECOND_TOKEN",
			"TEST_GITHUB_SECOND_EL_URL",
		); err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
	}

	var split []string
	if !viaDirectWebhook {
		split = strings.Split(githubRepoOwnerGithubApp, "/")
	}
	if viaDirectWebhook {
		githubToken = os.Getenv("TEST_GITHUB_TOKEN")
		split = strings.Split(githubRepoOwnerDirectWebhook, "/")
	}
	if onSecondController {
		githubURL = os.Getenv("TEST_GITHUB_SECOND_API_URL")
		githubRepoOwnerGithubApp = os.Getenv("TEST_GITHUB_SECOND_REPO_OWNER_GITHUBAPP")
		githubToken = os.Getenv("TEST_GITHUB_SECOND_TOKEN")
		controllerURL = os.Getenv("TEST_GITHUB_SECOND_EL_URL")
		split = strings.Split(githubRepoOwnerGithubApp, "/")
	}

	run := params.New()
	if err := run.Clients.NewClients(ctx, &run.Info); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}
	run.Info.Controller = info.GetControllerInfoFromEnvOrDefault()
	e2eoptions := options.E2E{Organization: split[0], Repo: split[1], DirectWebhook: viaDirectWebhook, ControllerURL: controllerURL}
	gprovider := github.New()
	gprovider.Run = run
	event := info.NewEvent()

	if githubToken == "" && !viaDirectWebhook {
		var err error

		ctx, err = cctx.GetControllerCtxInfo(ctx, run)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}

		envGithubRepoInstallationID, err := setup.GetRequiredEnv("TEST_GITHUB_REPO_INSTALLATION_ID")
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
		// convert to int64 githubRepoInstallationID
		githubRepoInstallationID, err := strconv.ParseInt(envGithubRepoInstallationID, 10, 64)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), fmt.Errorf("TEST_GITHUB_REPO_INSTALLATION_ID env variable must be an integer but got '%s'", envGithubRepoInstallationID)
		}
		ns := info.GetNS(ctx)
		githubToken, err = gprovider.GetAppToken(ctx, run.Clients.Kube, githubURL, githubRepoInstallationID, ns)
		if err != nil {
			return ctx, nil, options.E2E{}, github.New(), err
		}
	}

	event.Provider = &info.Provider{
		Token: githubToken,
		URL:   githubURL,
	}
	gprovider.Token = &githubToken
	// TODO: before PR
	if err := gprovider.SetClient(ctx, nil, event, nil, nil); err != nil {
		return ctx, nil, options.E2E{}, github.New(), err
	}

	return ctx, run, e2eoptions, gprovider, nil
}

func PatchPACConfigMap(ctx context.Context, run *params.Run, patch map[string]interface{}) (*corev1.ConfigMap, error) {
	ns, _, err := params.GetInstallLocation(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("failed to get Pipelines as Code namespace: %w", err)
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Fatalf("Failed to marshal patch data: %v", err)
	}

	cm, err := run.Clients.Kube.CoreV1().ConfigMaps(ns).Patch(ctx,
		run.Info.Controller.Configmap,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to patch Pipelines-as-Code config map: %w", err)
	}

	return cm, nil
}

func SetCancelInProgressToDefaults(ctx context.Context, run *params.Run) error {
	ns, _, err := params.GetInstallLocation(ctx, run)
	if err != nil {
		return fmt.Errorf("failed to get Pipelines as Code namespace: %w", err)
	}

	patch := map[string]interface{}{
		"data": map[string]string{
			"enable-cancel-in-progress-on-pull-requests": "false",
			"enable-cancel-in-progress-on-push":          "false",
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Fatalf("Failed to marshal patch data: %v", err)
	}

	_, err = run.Clients.Kube.CoreV1().ConfigMaps(ns).Patch(ctx,
		run.Info.Controller.Configmap,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch Pipelines-as-Code config map: %w", err)
	}

	return nil
}
