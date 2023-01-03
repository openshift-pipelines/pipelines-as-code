package reconciler

import (
	"context"
	"testing"

	provider2 "github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func TestCreateStatusWithRetry(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	vcx := provider.TestProviderImp{}

	err := createStatusWithRetry(context.TODO(), fakelogger, nil, &vcx, nil, nil, provider2.StatusOpts{})
	assert.NilError(t, err)
}

func TestCreateStatusWithRetry_ErrorCase(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	vcx := provider.TestProviderImp{}
	vcx.CreateStatusErorring = true

	err := createStatusWithRetry(context.TODO(), fakelogger, nil, &vcx, nil, nil, provider2.StatusOpts{})
	assert.Error(t, err, "failed to report status: some provider error occurred while reporting status")
}
