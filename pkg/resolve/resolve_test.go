package resolve

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/assert"
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
func readTDfile(testname string) (*v1beta1.PipelineRun, error) {
	data, err := ioutil.ReadFile("testdata/" + testname + ".yaml")
	if err != nil {
		return &v1beta1.PipelineRun{}, err
	}
	resolved, err := Resolve(data)
	if err != nil {
		return &v1beta1.PipelineRun{}, err
	}
	return resolved[0], nil
}

func TestPipelineRunPipelineTask(t *testing.T) {
	resolved, err := readTDfile("pipelinerun-pipeline-task")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "first-step")

	//TODO: we should do templates substitions for those values here?
	assert.Equal(t, resolved.Spec.Params[0].Value.StringVal, "{{value}}")
}

func TestPipelineRunPipelineSpecTaskSpec(t *testing.T) {
	resolved, err := readTDfile("pipelinerun-pipelinespec-taskspec")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "hello-moto")
}

func TestPipelineRunPipelineSpecTaskRef(t *testing.T) {
	t.Skip("TODO") // TODO: Not working ATM
	resolved, err := readTDfile("pipelinerun-pipelinespec-taskref")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "task1")
}

func TestPipelineRunPipelineRefTaskSpec(t *testing.T) {
	resolved, err := readTDfile("pipelinerun-pipelineref-taskspec")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "first-step")
}

func TestNoPipelineRuns(t *testing.T) {
	_, err := readTDfile("no-pipelinerun")
	assert.Error(t, err, "We need at least one pipelinerun to start with")
}

func TestReferencedTaskNotInRepo(t *testing.T) {
	_, err := readTDfile("referenced-task-not-in-repo")
	assert.Error(t, err, "Cannot find task nothere in input")
}

func TestReferencedPipelineNotInRepo(t *testing.T) {
	_, err := readTDfile("referenced-pipeline-not-in-repo")
	assert.Error(t, err, "Cannot find pipeline pipeline-test1 in input")
}
