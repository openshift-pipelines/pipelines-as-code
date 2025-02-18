package provider

import (
	"context"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
)

type StatusOpts struct {
	PipelineRun              *v1.PipelineRun
	PipelineRunName          string
	OriginalPipelineRunName  string
	Status                   string
	Conclusion               string
	Text                     string
	DetailsURL               string
	Summary                  string
	Title                    string
	InstanceCountForCheckRun int
	AccessDenied             bool
}

type Interface interface {
	SetLogger(*zap.SugaredLogger)
	Validate(ctx context.Context, params *params.Run, event *info.Event) error
	Detect(*http.Request, string, *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error)
	ParsePayload(context.Context, *params.Run, *http.Request, string) (*info.Event, error)
	IsAllowed(context.Context, *info.Event) (bool, error)
	IsAllowedOwnersFile(context.Context, *info.Event) (bool, error)
	CreateStatus(context.Context, *info.Event, StatusOpts) error
	GetTektonDir(context.Context, *info.Event, string, string) (string, error)      // ctx, event, path, provenance
	GetFileInsideRepo(context.Context, *info.Event, string, string) (string, error) // ctx, event, path, branch
	SetClient(context.Context, *params.Run, *info.Event, *v1alpha1.Repository, *events.EventEmitter) error
	SetPacInfo(*info.PacOpts)
	GetCommitInfo(context.Context, *info.Event) error
	GetConfig() *info.ProviderConfig
	GetFiles(context.Context, *info.Event) (changedfiles.ChangedFiles, error)
	GetTaskURI(ctx context.Context, event *info.Event, uri string) (bool, string, error)
	CreateToken(context.Context, []string, *info.Event) (string, error)
	CheckPolicyAllowing(context.Context, *info.Event, []string) (bool, string)
	GetTemplate(CommentType) string
	CreateComment(ctx context.Context, event *info.Event, comment, updateMarker string) error
}

const DefaultProviderAPIUser = "git"
