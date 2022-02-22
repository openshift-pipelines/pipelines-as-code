package resolve

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	rtesting "knative.dev/pkg/reconciler/testing"
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
func readTDfile(t *testing.T, testname string, generateName bool, remoteTasking bool) (*tektonv1beta1.PipelineRun, *zapobserver.ObservedLogs, error) {
	ctx, _ := rtesting.SetupFakeContext(t)
	data, err := ioutil.ReadFile("testdata/" + testname + ".yaml")
	if err != nil {
		return &tektonv1beta1.PipelineRun{}, nil, err
	}
	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cs := &params.Run{
		Clients: clients.Clients{
			Log: logger,
		},
		Info: info.Info{},
	}
	ropt := &Opts{
		GenerateName: generateName,
		RemoteTasks:  remoteTasking,
	}
	tprovider := &testprovider.TestProviderImp{}
	resolved, err := Resolve(ctx, cs, tprovider, string(data), ropt)
	if err != nil {
		return &tektonv1beta1.PipelineRun{}, nil, err
	}
	return resolved[0], log, nil
}

func TestPipelineRunPipelineTask(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "first-step")

	// TODO: we should do templates substitutions for those values here?
	assert.Equal(t, resolved.Spec.Params[0].Value.StringVal, "{{value}}")
}

func TestGenerateName(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task", true, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")

	resolved, _, err = readTDfile(t, "with-generatename", true, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")
}

func TestPipelineBundlesSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-bundle", false, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.Spec.PipelineRef.Bundle != "")
}

func TestTaskBundlesSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-task-bundle", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].Name, "bundled")
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Bundle, "reg.io/ruben/barichello@sha256:1234")
}

func TestClusterTasksSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-with-a-clustertasks", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].Name, "clustertask")
	assert.Equal(t, string(resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Kind), "ClusterTask")
}

func TestPipelineRunPipelineSpecTaskSpec(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipelinespec-taskspec", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "hello-moto")
}

func TestPipelineRunWithFinally(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-finally", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Finally[0].TaskSpec.Steps[0].Name, "finally-task")
}

func TestPipelineWithFinally(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipeline-finally", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "normal-task")
	assert.Equal(t, resolved.Spec.PipelineSpec.Finally[0].TaskSpec.Steps[0].Name, "finally-task")
}

func TestPipelineRunPipelineSpecTaskRef(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipelinespec-taskref", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps[0].Name, "task1")
}

func TestPipelineRunRemoteTaskDisabled(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task-remote-task-annotation", false, false)
	assert.NilError(t, err)
	// tbh: it would crash
	assert.Equal(t, resolved.GetAnnotations()["pipelinesascode.tekton.dev/task"], "foo")
}

func TestOriginalPRNameLabelSet(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task-remote-task-annotation", false, false)
	assert.NilError(t, err)
	assert.Equal(t, resolved.GetLabels()["pipelinesascode.tekton.dev/original-prname"], "pr")
}

func TestPipelineRunRemoteTaskNotPacAnnotations(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task-annotations-not-pac", false, true)
	assert.NilError(t, err)

	if _, ok := resolved.GetObjectMeta().GetAnnotations()["anno"]; !ok {
		t.Error("Did not get back the annotations")
	}
}

func TestPipelineRunRemoteTaskBadPacAnnotations(t *testing.T) {
	_, _, err := readTDfile(t, "pipelinerun-pipeline-task-bad-pac-annotation", false, true)
	assert.ErrorContains(t, err, "annotations in pipeline are in wrong format")
}

func TestNotTektonDocumentIgnore(t *testing.T) {
	resolved, log, err := readTDfile(t, "not-a-tekton-document", false, true)
	assert.NilError(t, err)
	logs := log.TakeAll()
	assert.Assert(t, len(logs) > 0)
	assert.Assert(t, strings.HasPrefix(logs[0].Message, "Skipping"))
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
}

func TestNotKubernetesDocumentIgnore(t *testing.T) {
	resolved, log, err := readTDfile(t, "not-a-kubernetes-yaml", false, true)
	logs := log.TakeAll()
	assert.Assert(t, len(logs) > 0)
	assert.Assert(t, strings.HasPrefix(logs[0].Message, "Skipping"))
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
	assert.NilError(t, err)
	assert.Assert(t, resolved.Spec.PipelineSpec != nil)
}

// test if we have the task in .tekton dir not referenced in annotations but taskRef in a task.
// should embed since in repo.
func TestInRepoShouldNotEmbedIfNoAnnotations(t *testing.T) {
	resolved, _, err := readTDfile(t, "in-repo-in-ref-no-annotation", false, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.Spec.PipelineSpec.Tasks[0].TaskRef == nil, "task should have been embedded")
}

func TestNoPipelineRuns(t *testing.T) {
	_, _, err := readTDfile(t, "no-pipelinerun", false, true)
	assert.Error(t, err, "we need at least one pipelinerun to start with")
}

func TestReferencedTaskNotInRepo(t *testing.T) {
	_, _, err := readTDfile(t, "referenced-task-not-in-repo", false, true)
	assert.Error(t, err, "cannot find task nothere in input")
}

func TestReferencedPipelineNotInRepo(t *testing.T) {
	_, _, err := readTDfile(t, "referenced-pipeline-not-in-repo", false, true)
	assert.Error(t, err, "cannot find pipeline pipeline-test1 in input")
}

func TestIgnoreDocSpace(t *testing.T) {
	_, _, err := readTDfile(t, "empty-spaces", false, true)
	assert.NilError(t, err)
}
