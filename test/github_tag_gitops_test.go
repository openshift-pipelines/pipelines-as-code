//go:build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v81/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/payload"
	"github.com/openshift-pipelines/pipelines-as-code/test/pkg/scm"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGithubGitOpsCommentOnTag(t *testing.T) {
	ctx := context.Background()
	ctx, runcnx, opts, ghcnx, err := tgithub.Setup(ctx, false, false)
	assert.NilError(t, err)
	tagName := "v1.0.0"
	comment := "/test tag:" + tagName
	sha := ""
	targetBranch := "release-" + tagName
	label := "Testing GitHub commit comment on tag with GitHub apps integration"
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-tag")
	numberOfPRs := 1
	repoinfo, resp, err := ghcnx.Client().Repositories.Get(ctx, opts.Organization, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Organization, opts.Repo)
	}
	err = tgithub.CreateCRD(ctx, t, repoinfo, runcnx, opts, targetNS)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Repository %s has been created successfully", targetNS)

	ref, resp, err := ghcnx.Client().Git.GetRef(ctx, opts.Organization, opts.Repo, "refs/tags/"+tagName)
	if err != nil && resp.StatusCode == http.StatusNotFound {
		runcnx.Clients.Log.Infof("Tag %s is not found on the repository", tagName)
		runcnx.Clients.Log.Infof("Creating tag %s on the repository", tagName)
		yamlFiles := []string{"testdata/pipelinerun-on-tag.yaml"}
		yamlEntries := map[string]string{}
		for _, v := range yamlFiles {
			yamlEntries[filepath.Join(".tekton", filepath.Base(v))] = v
		}

		entries, err := payload.GetEntries(yamlEntries,
			targetNS, "refs/tags/*", "push", map[string]string{})
		assert.NilError(t, err)

		targetRefName := targetBranch
		cloneURL, err := scm.MakeGitCloneURL(repoinfo.GetCloneURL(), "git", *ghcnx.Token)
		assert.NilError(t, err)
		scmOpts := scm.Opts{
			GitURL:        cloneURL,
			TargetRefName: targetRefName,
			BaseRefName:   repoinfo.GetDefaultBranch(),
			WebURL:        repoinfo.GetHTMLURL(),
			Log:           runcnx.Clients.Log,
			CommitTitle:   label,
		}
		scm.PushFilesToRefGit(t, &scmOpts, entries)
		branch, _, err := ghcnx.Client().Repositories.GetBranch(ctx, opts.Organization, opts.Repo, targetBranch, 1)
		assert.NilError(t, err)

		sha = branch.GetCommit().GetSHA()

		_, err = tgithub.CreateTag(ctx, t, runcnx, ghcnx, opts, sha, tagName)
		assert.NilError(t, err)
		numberOfPRs++
	} else {
		runcnx.Clients.Log.Infof("Tag %s is already created on the repository", tagName)
		runcnx.Clients.Log.Infof("Getting tag %s", tagName)
		tagRef, _, err := ghcnx.Client().Git.GetTag(ctx, opts.Organization, opts.Repo, ref.GetObject().GetSHA())
		assert.NilError(t, err)
		sha = tagRef.GetObject().GetSHA()
	}

	g := &tgithub.PRTest{
		Label:           label,
		Cnx:             runcnx,
		Logger:          runcnx.Clients.Log,
		Options:         opts,
		Provider:        ghcnx,
		TargetNamespace: targetNS,
		PRNumber:        -1,
		SHA:             sha,
	}
	defer g.TearDown(ctx, t)

	runcnx.Clients.Log.Infof("%s on tag commit", comment)
	_, _, err = ghcnx.Client().Repositories.CreateComment(ctx,
		opts.Organization,
		opts.Repo, sha, // this is the commit sha of the tag v1.0.0
		&github.RepositoryComment{Body: github.Ptr(comment)})
	assert.NilError(t, err)

	waitOpts := twait.Opts{
		RepoName:        targetNS,
		Namespace:       targetNS,
		MinNumberStatus: numberOfPRs,
		PollTimeout:     twait.DefaultTimeout,
		TargetSHA:       sha, // this is the commit sha of the tag v1.0.0
	}
	runcnx.Clients.Log.Info("Waiting for Repository to be updated")
	_, err = twait.UntilRepositoryUpdated(ctx, runcnx.Clients, waitOpts)
	assert.NilError(t, err)

	runcnx.Clients.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := runcnx.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, repo.Status[len(repo.Status)-1].Conditions[0].Status, corev1.ConditionTrue)

	pruns, err := runcnx.Clients.Tekton.TektonV1().PipelineRuns(targetNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", keys.SHA, sha),
	})
	assert.NilError(t, err)
	assert.Equal(t, len(pruns.Items), numberOfPRs)

	for i := range pruns.Items {
		sData, err := runcnx.Clients.Kube.CoreV1().Secrets(targetNS).Get(ctx, pruns.Items[i].GetAnnotations()[keys.GitAuthSecret], metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Assert(t, string(sData.Data["git-provider-token"]) != "")
		assert.Assert(t, string(sData.Data[".git-credentials"]) != "")
		assert.Assert(t, string(sData.Data[".gitconfig"]) != "")
	}
}
