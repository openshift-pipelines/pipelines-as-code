package adapter

import (
	"fmt"
	"net/http"
	"testing"

	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
)

func getLogger() *zap.SugaredLogger {
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	return logger
}

func TestWhichProvider(t *testing.T) {
	l := listener{
		logger: getLogger(),
	}
	tests := []struct {
		name    string
		request *http.Request
	}{
		{
			name: "github payload",
			request: &http.Request{
				Header: map[string][]string{
					"X-Github-Event":    []string{"pull_request"},
					"X-GitHub-Delivery": []string{"abcd"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := l.whichProvider(tt.request)
			assert.NilError(t, err)
		})
	}
}

func TestWhichProviderError(t *testing.T) {
	l := listener{
		logger: getLogger(),
	}
	tests := []struct {
		name    string
		request *http.Request
		err     error
	}{
		{
			name: "invalid payload",
			request: &http.Request{
				Header: map[string][]string{
					"X-Event": []string{"pull_request"},
				},
			},
			err: fmt.Errorf("no supported Git Provider is detected"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := l.whichProvider(tt.request)
			assert.Equal(t, err.Error(), tt.err.Error(), fmt.Sprintf("expected %v but got %v", tt.err, err))
		})
	}
}
