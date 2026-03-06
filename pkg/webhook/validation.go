package webhook

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	pac "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/listers/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/webhook"
)

var universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

var (
	allowedGitlabDisableCommentStrategyOnMr = sets.NewString("", provider.DisableAllCommentStrategy, provider.UpdateCommentStrategy)
	allowedForgejoCommentStrategyOnPr       = sets.NewString("", provider.DisableAllCommentStrategy, provider.UpdateCommentStrategy)
)

// Path implements AdmissionController.
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController.
func (ac *reconciler) Admit(_ context.Context, request *v1.AdmissionRequest) *v1.AdmissionResponse {
	raw := request.Object.Raw
	repo := v1alpha1.Repository{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &repo); err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	// Check that if we have a URL set only for non global repository which can be set as empty.
	if repo.GetNamespace() != os.Getenv("SYSTEM_NAMESPACE") {
		if repo.Spec.URL == "" {
			return webhook.MakeErrorStatus("URL must be set")
		}

		providerType := ""
		if repo.Spec.GitProvider != nil && repo.Spec.GitProvider.Type != "" {
			providerType = repo.Spec.GitProvider.Type
		}

		if err := validateRepositoryURL(repo.Spec.URL, providerType); err != nil {
			return webhook.MakeErrorStatus("%s", err.Error())
		}
	}

	exist, err := checkIfRepoExist(ac.pacLister, &repo, "")
	if err != nil {
		return webhook.MakeErrorStatus("validation failed: %v", err)
	}

	if exist {
		return webhook.MakeErrorStatus("repository already exists with URL: %s", repo.Spec.URL)
	}

	if repo.Spec.ConcurrencyLimit != nil && *repo.Spec.ConcurrencyLimit == 0 {
		return webhook.MakeErrorStatus("concurrency limit must be greater than 0")
	}

	if repo.Spec.Settings != nil && repo.Spec.Settings.Gitlab != nil {
		if !allowedGitlabDisableCommentStrategyOnMr.Has(repo.Spec.Settings.Gitlab.CommentStrategy) {
			return webhook.MakeErrorStatus("comment strategy '%s' is not supported for Gitlab MRs", repo.Spec.Settings.Gitlab.CommentStrategy)
		}
	}

	if repo.Spec.Settings != nil && repo.Spec.Settings.Forgejo != nil {
		if !allowedForgejoCommentStrategyOnPr.Has(repo.Spec.Settings.Forgejo.CommentStrategy) {
			return webhook.MakeErrorStatus("comment strategy '%s' is not supported for Forgejo/Gitea PRs", repo.Spec.Settings.Forgejo.CommentStrategy)
		}
	}

	return &v1.AdmissionResponse{Allowed: true}
}

func checkIfRepoExist(pac pac.RepositoryLister, repo *v1alpha1.Repository, ns string) (bool, error) {
	repositories, err := pac.Repositories(ns).List(labels.NewSelector())
	if err != nil {
		return false, err
	}
	for i := len(repositories) - 1; i >= 0; i-- {
		repoFromCluster := repositories[i]
		if repoFromCluster.Spec.URL == repo.Spec.URL &&
			(repoFromCluster.Name != repo.Name || repoFromCluster.Namespace != repo.Namespace) {
			return true, nil
		}
	}
	return false, nil
}

func validateRepositoryURL(repoURL, providerType string) error {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	// Strict validation ensures that the GitHub repository URL does not include
	// additional path segments (e.g., https://github.com/org/repo/extra).
	// However, we cannot reliably determine whether a self-hosted GitHub or
	// GHE instance should be classified as type "github". Therefore, strict
	// validation is applied only when we can confidently identify the repository
	// as a GitHub repository.
	if parsedURL.Host == "github.com" {
		providerType = "github"
	}

	// GitHub doesn't support subgroups, so validate strictly for GitHub repos
	shouldValidateStrictly := providerType == "github"

	if shouldValidateStrictly {
		pathSegments := []string{}
		for _, seg := range strings.Split(strings.Trim(parsedURL.Path, "/"), "/") {
			if seg != "" {
				pathSegments = append(pathSegments, seg)
			}
		}

		if len(pathSegments) != 2 {
			return fmt.Errorf("GitHub repository URL must follow https://github.com/org/repo format without subgroups (found %d path segments, expected 2)", len(pathSegments))
		}
	}

	return nil
}
