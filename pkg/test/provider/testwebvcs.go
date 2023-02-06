package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

var _ provider.Interface = (*TestProviderImp)(nil)

type TestProviderImp struct {
	AllowIT                bool
	Event                  *info.Event
	TektonDirTemplate      string
	CreateStatusErorring   bool
	FilesInsideRepo        map[string]string
	WantProviderRemoteTask bool
}

func (v *TestProviderImp) SetLogger(logger *zap.SugaredLogger) {
}

func (v *TestProviderImp) Validate(ctx context.Context, params *params.Run, event *info.Event) error {
	return nil
}

func (v *TestProviderImp) Detect(request *http.Request, body string, logger *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	return true, true, nil, "", nil
}

func (v *TestProviderImp) ParsePayload(ctx context.Context, run *params.Run, request *http.Request, payload string) (*info.Event, error) {
	return v.Event, nil
}

func (v *TestProviderImp) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{}
}

func (v *TestProviderImp) GetCommitInfo(ctx context.Context, runevent *info.Event) error {
	return nil
}

func (v *TestProviderImp) SetClient(ctx context.Context, run *params.Run, event *info.Event) error {
	return nil
}

func (v *TestProviderImp) IsAllowed(ctx context.Context, event *info.Event) (bool, error) {
	if v.AllowIT {
		return true, nil
	}
	return false, nil
}

func (v *TestProviderImp) GetTaskURI(ctx context.Context, params *params.Run, event *info.Event, task string) (bool, string, error) {
	return v.WantProviderRemoteTask, "", nil
}

func (v *TestProviderImp) CreateStatus(ctx context.Context, _ versioned.Interface, event *info.Event, opts *info.PacOpts, statusOpts provider.StatusOpts) error {
	if v.CreateStatusErorring {
		return fmt.Errorf("some provider error occurred while reporting status")
	}
	return nil
}

func (v *TestProviderImp) GetTektonDir(ctx context.Context, event *info.Event, s string) (string, error) {
	return v.TektonDirTemplate, nil
}

func (v *TestProviderImp) GetFileInsideRepo(ctx context.Context, event *info.Event, file, targetBranch string) (string, error) {
	if val, ok := v.FilesInsideRepo[file]; ok {
		return val, nil
	}
	return "", fmt.Errorf("could not find %s in tests", file)
}

func (v *TestProviderImp) GetFiles(ctx context.Context, event *info.Event) ([]string, error) {
	return []string{}, nil
}

func (v *TestProviderImp) ListRepos(ctx context.Context) ([]string, error) {
	return []string{}, nil
}
