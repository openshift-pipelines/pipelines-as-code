package scheduler

import (
	"net/http"

	pacClientSet "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	pipelineClientSet "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

const (
	RepoNameAnnotation = "pipelinesascode.tekton.dev/repositoryName"
)

type Scheduler interface {
	Register() http.HandlerFunc
}

type scheduler struct {
	pipelineClient pipelineClientSet.Interface
	pacClient      pacClientSet.Interface
	logger         *zap.SugaredLogger
}

var _ Scheduler = &scheduler{}

func New(p pipelineClientSet.Interface, pac pacClientSet.Interface, l *zap.SugaredLogger) Scheduler {
	return &scheduler{
		pipelineClient: p,
		pacClient:      pac,
		logger:         l,
	}
}
