package resolve

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
)

func TestMain(m *testing.M) {

	setup()
	ret := m.Run()
	os.Exit(ret)
}

func setup() {
	s := k8scheme.Scheme
	if err := tektonv1beta1.AddToScheme(s); err != nil {
		log.Fatalf("Unable to add route scheme: (%v)", err)
	}
}

// Not sure how to get testParams fixtures working
func readTDfile(testname string, generateName bool) (*v1beta1.PipelineRun, *zapobserver.ObservedLogs, error) {
	data, err := ioutil.ReadFile("testdata/" + testname + ".yaml")
	if err != nil {
		return &v1beta1.PipelineRun{}, nil, err
	}
	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cs := &cli.Clients{
		Log: logger,
	}
	resolved, err := resolve(cs, data, generateName)
	if err != nil {
		return &v1beta1.PipelineRun{}, nil, err
	}
	return resolved[0], log, nil
}

func TestPipelineRunPipelineTask(t *testing.T) {
	resolved, _, err := readTDfile("pipelinerun-pipeline-task", false)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "first-step")

	//TODO: we should do templates substitions for those values here?
	assert.Equal(t, resolved.Spec.Params[0].Value.StringVal, "{{value}}")
}

func TestGenerateName(t *testing.T) {
	resolved, _, err := readTDfile("pipelinerun-pipeline-task", true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")

	resolved, _, err = readTDfile("with-generatename", true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")
}

func TestPipelineRunPipelineSpecTaskSpec(t *testing.T) {
	resolved, _, err := readTDfile("pipelinerun-pipelinespec-taskspec", false)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "hello-moto")
}

func TestPipelineRunPipelineSpecTaskRef(t *testing.T) {
	resolved, _, err := readTDfile("pipelinerun-pipelinespec-taskref", false)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "task1")
}

func TestNotTektonDocumentIgnore(t *testing.T) {
	resolved, log, err := readTDfile("not-a-tekton-document", false)
	assert.NilError(t, err)
	logs := log.TakeAll()
	assert.Assert(t, len(logs) > 0)
	assert.Assert(t, strings.HasPrefix(logs[0].Message, "Skipping"))
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
}

func TestNotKubernetesDocumentIgnore(t *testing.T) {
	resolved, log, err := readTDfile("not-a-kubernetes-yaml", false)
	logs := log.TakeAll()
	assert.Assert(t, len(logs) > 0)
	assert.Assert(t, strings.HasPrefix(logs[0].Message, "Skipping"))
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
	assert.NilError(t, err)
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
}

func TestNoPipelineRuns(t *testing.T) {
	_, _, err := readTDfile("no-pipelinerun", false)
	assert.Error(t, err, "We need at least one pipelinerun to start with")
}

func TestReferencedTaskNotInRepo(t *testing.T) {
	_, _, err := readTDfile("referenced-task-not-in-repo", false)
	assert.Error(t, err, "Cannot find task nothere in input")
}

func TestReferencedPipelineNotInRepo(t *testing.T) {
	_, _, err := readTDfile("referenced-pipeline-not-in-repo", false)
	assert.Error(t, err, "Cannot find pipeline pipeline-test1 in input")
}

func TestIgnoreDocSpace(t *testing.T) {
	_, _, err := readTDfile("empty-spaces", false)
	assert.NilError(t, err)
}
