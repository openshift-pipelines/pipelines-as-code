package formatting

import (
	"testing"
)

func TestMessageTemplate_MakeTemplate(t *testing.T) {
	mt := MessageTemplate{
		PipelineRunName: "test-pipeline",
		Namespace:       "test-namespace",
		ConsoleName:     "test-console",
		ConsoleURL:      "https://test-console-url.com",
		TknBinary:       "test-tkn",
		TknBinaryURL:    "https://test-tkn-url.com",
		FailureSnippet:  "such a failure",
	}

	tests := []struct {
		name    string
		mt      MessageTemplate
		msg     string
		want    string
		wantErr bool
	}{
		{
			name: "Test MakeTemplate",
			mt:   mt,
			msg:  "Starting Pipelinerun {{.Mt.PipelineRunName}} in namespace {{.Mt.Namespace}}",
			want: "Starting Pipelinerun test-pipeline in namespace test-namespace",
		},
		{
			name:    "Error MakeTemplate",
			mt:      mt,
			msg:     "Starting Pipelinerun {{.Mt.PipelineRunName}} in namespace {{.FOOOBAR }}",
			wantErr: true,
		},
		{
			name: "Failure template",
			mt:   mt,
			msg:  "I am {{ .Mt.FailureSnippet }}",
			want: "I am such a failure",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mt.MakeTemplate(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("MessageTemplate.MakeTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MessageTemplate.MakeTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
