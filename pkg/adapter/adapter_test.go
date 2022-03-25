package adapter

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v43/github"
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
		name          string
		event         interface{}
		header        http.Header
		wantErrString string
	}{
		{
			name: "github event",
			header: map[string][]string{
				"X-Github-Event":    {"push"},
				"X-GitHub-Delivery": {"abcd"},
			},
			event: github.PushEvent{
				Pusher: &github.User{ID: github.Int64(123)},
			},
		},
		{
			name: "some random event",
			header: map[string][]string{
				"foo": {"bar"},
			},
			event:         "interface",
			wantErrString: "no supported Git Provider is detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jeez, err := json.Marshal(tt.event)
			if err != nil {
				assert.NilError(t, err)
			}

			_, _, err = l.detectProvider(&tt.header, string(jeez))
			if tt.wantErrString != "" {
				assert.ErrorContains(t, err, tt.wantErrString)
				return
			}
			assert.NilError(t, err)
		})
	}
}
