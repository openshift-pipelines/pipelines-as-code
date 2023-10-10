package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	tgitea "github.com/openshift-pipelines/pipelines-as-code/test/pkg/gitea"
	pacrepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Args struct {
	name      *string
	onOrg     *bool
	targetNS  *string
	localRepo *string
}

func create(ctx context.Context, args *Args) error {
	run, e2eOpts, provider, err := tgitea.Setup(ctx)
	if err != nil {
		return err
	}
	hookURL := os.Getenv("TEST_GITEA_SMEEURL")

	if _, err = provider.Client.DeleteRepo(e2eOpts.Organization, *args.name); err == nil {
		run.Clients.Log.Infof("repository %s/%s deleted", e2eOpts.Organization, *args.name)
	}

	repoInfo, err := tgitea.CreateGiteaRepo(
		provider.Client,
		e2eOpts.Organization,
		*args.name,
		hookURL,
		*args.onOrg,
		run.Clients.Log)
	if err != nil {
		return err
	}

	parsed, _ := url.Parse(repoInfo.HTMLURL)
	userPasswordURL := fmt.Sprintf("http://pac:%s@%s%s", *provider.Token, parsed.Host, parsed.Path)

	targetNS := *args.targetNS
	if targetNS == "" {
		targetNS = *args.name
	}

	topts := &tgitea.TestOpts{
		ParamsRun:        run,
		TargetNS:         targetNS,
		InternalGiteaURL: os.Getenv("TEST_GITEA_INTERNAL_URL"),
	}
	if topts.InternalGiteaURL == "" {
		topts.InternalGiteaURL = "http://gitea.gitea:3000"
	}

	_ = pacrepo.CreateNS(ctx, topts.TargetNS, topts.ParamsRun)
	err = run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(topts.TargetNS).Delete(
		ctx, targetNS, metav1.DeleteOptions{})
	if err == nil {
		run.Clients.Log.Infof("repository %s/%s deleted", topts.TargetNS, targetNS)
	}
	_ = run.Clients.Kube.CoreV1().Secrets(topts.TargetNS).Delete(ctx, "gitea-secret", metav1.DeleteOptions{})
	if err := tgitea.CreateCRD(ctx, topts); err != nil {
		return err
	}

	localRepoCheckout := "/tmp/" + *args.name
	if *args.localRepo != "" {
		localRepoCheckout = *args.localRepo
	}

	// check  if directory exist
	if _, err := os.Stat(localRepoCheckout); !os.IsNotExist(err) {
		return nil
	}

	_, _ = git.RunGit("/tmp", "clone", userPasswordURL, localRepoCheckout)
	if err := os.Chdir(localRepoCheckout); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, "tkn", "pac", "generate", "--event-type", "pull_request", "--branch", "main").Run(); err != nil {
		return err
	}

	_, _ = git.RunGit(localRepoCheckout, "checkout", "-b", "tektonci")
	_, _ = git.RunGit(localRepoCheckout, "commit", "-am", "Tekton FTW")

	fmt.Fprintf(os.Stdout, "Local Checkout Directory: %s\n", localRepoCheckout)
	fmt.Fprintf(os.Stdout, "Gitea Repositoyr URL: %s\n", repoInfo.HTMLURL)
	return nil
}

func main() {
	// check for args repo name was passed
	opts := &Args{}
	opts.name = flag.String("repo", "test-repo", "repository name to create")
	opts.targetNS = flag.String("targetNS", "", "namespace target")
	opts.localRepo = flag.String("localRepo", "", "name of the local repo to clone")
	opts.onOrg = flag.Bool("onOrg", false, "create repo on organization")
	flag.Parse()
	ctx := context.Background()
	if err := create(ctx, opts); err != nil {
		fmt.Fprintf(os.Stdout, "Error: %v\n", err)
	}
}
