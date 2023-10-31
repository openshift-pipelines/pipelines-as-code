package formatting

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/starting.go.tmpl
var StartingPipelineRunText string

//go:embed templates/queuing.go.tmpl
var QueuingPipelineRunText string

//go:embed templates/pipelinerunstatus.tmpl
var PipelineRunStatusText string

type MessageTemplate struct {
	PipelineRunName string
	Namespace       string
	NamespaceURL    string
	ConsoleName     string
	ConsoleURL      string
	TknBinary       string
	TknBinaryURL    string
	TaskStatus      string
	FailureSnippet  string
}

func (mt MessageTemplate) MakeTemplate(tmpl string) (string, error) {
	outputBuffer := bytes.Buffer{}
	t := template.Must(template.New("Message").Parse(tmpl))
	data := struct{ Mt MessageTemplate }{Mt: mt}
	if err := t.Execute(&outputBuffer, data); err != nil {
		return "", err
	}
	return outputBuffer.String(), nil
}
