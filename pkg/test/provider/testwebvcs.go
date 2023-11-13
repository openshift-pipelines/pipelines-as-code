package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/events"
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
	PolicyDisallowing      bool
	AllowedInOwnersFile    bool
}

func (v *TestProviderImp) CheckPolicyAllowing(_ context.Context, _ *info.Event, _ []string) (bool, string) {
	if v.PolicyDisallowing {
		return false, "policy disallowing"
	}
	return true, ""
}

func (v *TestProviderImp) IsAllowedOwnersFile(_ context.Context, _ *info.Event) (bool, error) {
	return v.AllowedInOwnersFile, nil
}

func (v *TestProviderImp) SetLogger(_ *zap.SugaredLogger) {
}

func (v *TestProviderImp) Validate(_ context.Context, _ *params.Run, _ *info.Event) error {
	return nil
}

func (v *TestProviderImp) Detect(_ *http.Request, _ string, _ *zap.SugaredLogger) (bool, bool, *zap.SugaredLogger, string, error) {
	return true, true, nil, "", nil
}

func (v *TestProviderImp) ParsePayload(_ context.Context, _ *params.Run, _ *http.Request, _ string) (*info.Event, error) {
	return v.Event, nil
}

func (v *TestProviderImp) GetConfig() *info.ProviderConfig {
	return &info.ProviderConfig{}
}

func (v *TestProviderImp) GetCommitInfo(_ context.Context, _ *info.Event) error {
	return nil
}

func (v *TestProviderImp) SetClient(_ context.Context, _ *params.Run, _ *info.Event, _ *v1alpha1.Repository, _ *events.EventEmitter) error {
	return nil
}

func (v *TestProviderImp) IsAllowed(_ context.Context, _ *info.Event, _ *info.PacOpts) (bool, error) {
	if v.AllowIT {
		return true, nil
	}
	return false, nil
}

func (v *TestProviderImp) GetTaskURI(_ context.Context, _ *params.Run, _ *info.Event, _ string) (bool, string, error) {
	return v.WantProviderRemoteTask, "", nil
}

func (v *TestProviderImp) CreateStatus(_ context.Context, _ versioned.Interface, _ *info.Event, _ *info.PacOpts, _ provider.StatusOpts) error {
	if v.CreateStatusErorring {
		return fmt.Errorf("some provider error occurred while reporting status")
	}
	return nil
}

func (v *TestProviderImp) GetTektonDir(_ context.Context, _ *info.Event, _, _ string) (string, error) {
	return v.TektonDirTemplate, nil
}

func (v *TestProviderImp) GetFileInsideRepo(_ context.Context, _ *info.Event, file, _ string) (string, error) {
	if val, ok := v.FilesInsideRepo[file]; ok {
		return val, nil
	}
	return "", fmt.Errorf("could not find %s in tests", file)
}

func (v *TestProviderImp) GetFiles(_ context.Context, _ *info.Event) ([]string, error) {
	return []string{}, nil
}

func (v *TestProviderImp) CreateToken(_ context.Context, _ []string, _ *params.Run, _ *info.Event) (string, error) {
	return "", nil
}
