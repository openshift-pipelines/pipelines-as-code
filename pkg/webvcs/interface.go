package webvcs

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
)

type StatusOpts struct {
	PipelineRunName string
	Status          string
	Conclusion      string
	Text            string
	DetailsURL      string
	Summary         string
	Title           string
}

type Interface interface {
	ParsePayload(context.Context, *params.Run, string) (*info.Event, error)
	IsAllowed(context.Context, *info.Event) (bool, error)
	CreateStatus(context.Context, *info.Event, *info.PacOpts, StatusOpts) error
	GetTektonDir(context.Context, *info.Event, string) (string, error)              // ctx, event, path
	GetFileInsideRepo(context.Context, *info.Event, string, string) (string, error) // ctx, event, path, branch
	SetClient(context.Context, *info.PacOpts) error
	GetCommitInfo(context.Context, *info.Event) error
	GetConfig() *info.VCSConfig
}
