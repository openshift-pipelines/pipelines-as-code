package kubeinteraction

import (
	"bytes"
	"io"
	"os"

	"github.com/jonboulle/clockwork"
	cliinterface "github.com/tektoncd/cli/pkg/cli"
	clilog "github.com/tektoncd/cli/pkg/log"
	cliprdesc "github.com/tektoncd/cli/pkg/pipelinerun/description"
	k8s "k8s.io/client-go/kubernetes"
)

type SidestepTektonCliParams struct {
	clients cliinterface.Clients
	ns      string
}

// SetKubeConfigPath uses the kubeconfig path to instantiate tekton
// returned by Clientset function
func (s SidestepTektonCliParams) SetKubeConfigPath(string) {
}

// SetKubeContext extends the specificity of the above SetKubeConfigPath
// by using a context other than the default context in the given kubeconfig
func (s SidestepTektonCliParams) SetKubeContext(string) {
}

func (s SidestepTektonCliParams) Clients() (*cliinterface.Clients, error) {
	return &s.clients, nil
}

func (s SidestepTektonCliParams) KubeClient() (k8s.Interface, error) {
	return nil, nil
}

// SetNamespace can be used to store the namespace parameter that is needed
// by most commands
func (s SidestepTektonCliParams) SetNamespace(ns string) {
	s.ns = ns
}

func (s SidestepTektonCliParams) Namespace() string {
	return s.ns
}

// SetNoColour set colouring or not
func (s SidestepTektonCliParams) SetNoColour(bool) {
}

func (s SidestepTektonCliParams) Time() clockwork.Clock {
	return nil
}

func (k Interaction) TektonCliFollowLogs(namespace, prName string) (string, error) {
	var outputBuffer bytes.Buffer

	k.TektonCliLogsOptions.Params.SetNamespace(namespace)
	k.TektonCliLogsOptions.PipelineRunName = prName
	lr, err := clilog.NewReader(clilog.LogTypePipeline, &k.TektonCliLogsOptions)
	if err != nil {
		return "", err
	}
	logC, errC, err := lr.Read()
	if err != nil {
		return "", err
	}

	mwr := io.MultiWriter(os.Stdout, &outputBuffer)

	k.TektonCliLogsOptions.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}

	clilog.NewWriter(clilog.LogTypePipeline).Write(k.TektonCliLogsOptions.Stream, logC, errC)
	return outputBuffer.String(), nil
}

// TektonCliPRDescribe Will use Tekton CLI to get a description of a PR in a namespace
func (k Interaction) TektonCliPRDescribe(prName, namespace string) (string, error) {
	var outputBuffer bytes.Buffer
	k.TektonCliLogsOptions.Params.SetNamespace(namespace)
	k.TektonCliLogsOptions.PipelineRunName = prName
	mwr := io.MultiWriter(os.Stdout, &outputBuffer)
	k.TektonCliLogsOptions.Stream = &cliinterface.Stream{
		Out: mwr,
		Err: mwr,
	}
	err := cliprdesc.PrintPipelineRunDescription(k.TektonCliLogsOptions.Stream, prName, k.TektonCliLogsOptions.Params)
	return outputBuffer.String(), err
}
