package provider

import (
	"context"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

type StatusOpts struct {
	PipelineRun             *v1beta1.PipelineRun
	PipelineRunName         string
	OriginalPipelineRunName string
	Status                  string
	Conclusion              string
	Text                    string
	DetailsURL              string
	Summary                 string
	Title                   string
}

type Interface interface {
	SetLogger(*zap.SugaredLogger)
	Validate(ctx context.Context, params *params.Run, event *info.Event) error
	Detect(*http.Request, string, *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error)
	ParsePayload(context.Context, *params.Run, *http.Request, string) (*info.Event, error)
	IsAllowed(context.Context, *info.Event) (bool, error)
	CreateStatus(context.Context, versioned.Interface, *info.Event, *info.PacOpts, StatusOpts) error
	GetTektonDir(context.Context, *info.Event, string) (string, error)              // ctx, event, path
	GetFileInsideRepo(context.Context, *info.Event, string, string) (string, error) // ctx, event, path, branch
	SetClient(context.Context, *info.Event) error
	GetCommitInfo(context.Context, *info.Event) error
	GetConfig() *info.ProviderConfig
	GetFiles(context.Context, *info.Event) ([]string, error)
}

const DefaultProviderAPIUser = "git"
