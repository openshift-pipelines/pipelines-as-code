package webvcs

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"go.uber.org/zap"
)

type StatusOpts struct {
	Status     string
	Conclusion string
	Text       string
	DetailsURL string
	Summary    string
	Title      string
}

type Interface interface {
	ParsePayload(context.Context, *zap.SugaredLogger, *info.Event, string) (*info.Event, error)
	IsAllowed(context.Context, *info.Event) (bool, error)
	CreateStatus(context.Context, *info.Event, info.PacOpts, StatusOpts) error
	GetTektonDir(context.Context, *info.Event, string) (string, error)            // ctx, event, path
	GetFileInsideRepo(context.Context, *info.Event, string, bool) (string, error) // ctx, event, path, branch
	SetClient(context.Context, info.PacOpts)
	GetCommitInfo(context.Context, *info.Event) error
}
