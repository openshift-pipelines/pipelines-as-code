package sync

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	RepoNameAnnotation = "pipelinesascode.tekton.dev/repositoryName"
)

type LockName struct {
	Namespace string
	RepoName  string
}

func NewLockName(repoName, namespace string) *LockName {
	return &LockName{
		RepoName:  repoName,
		Namespace: namespace,
	}
}

func GetLockName(pr *v1beta1.PipelineRun) *LockName {
	repoName := pr.GetAnnotations()[RepoNameAnnotation]
	return NewLockName(repoName, pr.Namespace)
}

func (ln *LockName) LockKey() string {
	return fmt.Sprintf("%s/%s", ln.Namespace, ln.RepoName)
}

func HolderKey(pr *v1beta1.PipelineRun) string {
	return fmt.Sprintf("%s/%s", pr.Namespace, pr.Name)
}

func (ln *LockName) DecodeLockName(repoKey string) *LockName {
	items := strings.Split(repoKey, "/")
	return &LockName{Namespace: items[0], RepoName: items[1]}
}
