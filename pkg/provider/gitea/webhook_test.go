package gitea

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_parseWebhook(t *testing.T) {
	type args struct {
		eventType   whEventType
		payloadFile string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "push",
			args: args{
				eventType:   EventTypeCreate,
				payloadFile: "testdata/push.json",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := os.ReadFile(tt.args.payloadFile)
			assert.NilError(t, err)
			_, err = parseWebhook(tt.args.eventType, payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWebhook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
