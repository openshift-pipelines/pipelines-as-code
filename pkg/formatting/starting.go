package formatting

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed templates/starting.go.tmpl
var StartingPipelineRunHTML string

//go:embed templates/starting.markdown.go.tmpl
var StartingPipelineRunMarkdown string

//go:embed templates/queuing.go.tmpl
var QueuingPipelineRunHTML string

//go:embed templates/queuing.markdown.go.tmpl
var QueuingPipelineRunMarkdown string

//go:embed templates/pipelinerunstatus.tmpl
var PipelineRunStatusHTML string

//go:embed templates/pipelinerunstatus_markdown.tmpl
var PipelineRunStatusMarkDown string

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
