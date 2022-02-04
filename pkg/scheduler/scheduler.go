package scheduler

import (
	"net/http"

	pacClientSet "github.com/openshift-pipelines/pipelines-as-code/pkg/generated/clientset/versioned"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/sync"
	pipelineClientSet "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

const (
	RepoAnnotation = "pipelinesascode.tekton.dev/repository"
)

type Scheduler interface {
	Register() http.HandlerFunc
}

type scheduler struct {
	syncManager    *sync.Manager
	pipelineClient pipelineClientSet.Interface
	pacClient      pacClientSet.Interface
	logger         *zap.SugaredLogger
}

var _ Scheduler = &scheduler{}

func New(m *sync.Manager, p pipelineClientSet.Interface, pac pacClientSet.Interface, l *zap.SugaredLogger) Scheduler {
	return &scheduler{
		syncManager:    m,
		pipelineClient: p,
		pacClient:      pac,
		logger:         l,
	}
}
