package webvcs

import (
	"context"
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"go.uber.org/zap"
)

type TestWebVCSImp struct {
	AllowIT              bool
	Event                *info.Event
	TektonDirTemplate    string
	CreateStatusErorring bool
	FilesInsideRepo      map[string]string
}

func (v *TestWebVCSImp) ParsePayload(ctx context.Context, logger *zap.SugaredLogger, event *info.Event,
	s string) (*info.Event, error) {
	return v.Event, nil
}

func (v *TestWebVCSImp) GetCommitInfo(ctx context.Context, runevent *info.Event) error {
	return nil
}

func (v *TestWebVCSImp) SetClient(ctx context.Context, pacopt info.PacOpts) {
}

func (v *TestWebVCSImp) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	if v.AllowIT {
		return true, nil
	}
	return false, nil
}

func (v *TestWebVCSImp) CreateStatus(ctx context.Context, event *info.Event, opts info.PacOpts,
	statusOpts webvcs.StatusOpts) error {
	if v.CreateStatusErorring {
		return fmt.Errorf("you want me to error I error for you")
	}
	return nil
}

func (v *TestWebVCSImp) GetTektonDir(ctx context.Context, event *info.Event, s string) (string, error) {
	return v.TektonDirTemplate, nil
}

func (v *TestWebVCSImp) GetFileInsideRepo(ctx context.Context, event *info.Event, s string, b bool) (string, error) {
	if val, ok := v.FilesInsideRepo[s]; ok {
		return val, nil
	}
	return "", fmt.Errorf("could not find %s in tests", s)
}
