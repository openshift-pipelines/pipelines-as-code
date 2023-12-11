package resolve

import (
	"log"
	"os"
	"strings"
	"testing"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if err := tektonv1.AddToScheme(s); err != nil {
		log.Fatalf("Unable to add tekton v1 scheme: (%v)", err)
	}
}

// Not sure how to get testParams fixtures working.
func readTDfile(t *testing.T, testname string, generateName, remoteTasking bool) (*tektonv1.PipelineRun, *zapobserver.ObservedLogs, error) {
	t.Helper()
	ctx, _ := rtesting.SetupFakeContext(t)
	data, err := os.ReadFile("testdata/" + testname + ".yaml")
	if err != nil {
		return &tektonv1.PipelineRun{}, nil, err
	}
	observer, log := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	cs := &params.Run{
		Clients: clients.Clients{},
		Info:    info.Info{},
	}
	ropt := &Opts{
		GenerateName: generateName,
		RemoteTasks:  remoteTasking,
	}
	event := &info.Event{}
	tprovider := &testprovider.TestProviderImp{}

	types, err := ReadTektonTypes(ctx, logger, string(data))
	if err != nil {
		return &tektonv1.PipelineRun{}, nil, err
	}
	resolved, err := Resolve(ctx, cs, logger, tprovider, types, event, ropt)
	if err != nil {
		return &tektonv1.PipelineRun{}, nil, err
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

func TestPipelineRunPipelineMiddle(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-splitted-with-dashes-in-middle", false, true)
	assert.NilError(t, err)
	assert.Equal(t, len(resolved.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps), 2, "we had the split bug on yaml multidocument ---")
}

func TestGenerateName(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-task", true, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")

	resolved, _, err = readTDfile(t, "with-generatename", true, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.ObjectMeta.GenerateName != "")
}

// TestPipelineBundlesSkipped effectively test conversion from beta1 to v1.
func TestPipelineBundlesSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipeline-bundle", false, true)
	assert.NilError(t, err)
	assert.Equal(t, string(resolved.Spec.PipelineRef.ResolverRef.Params[0].Name), "bundle")
}

// TestPipelineBundlesSkipped effectively test conversion from beta1 to v1.
func TestTaskBundlesSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-task-bundle", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].Name, "bundled")
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Params[0].Name, "bundle")
}

func TestTaskResolverSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-task-resolver", false, true)
	assert.NilError(t, err)
	assert.Assert(t, resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Resolver == "resolver")
}

func TestPipelineResolverSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-pipelinerun-resolver", false, true)
	assert.NilError(t, err)
	assert.Equal(t, string(resolved.Spec.PipelineRef.Resolver), "resolver")
	assert.Equal(t, resolved.Spec.PipelineRef.Params[0].Value.StringVal, "task")
}

func TestClusterTasksSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-with-a-clustertasks", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].Name, "clustertask")
	assert.Equal(t, string(resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Kind), "ClusterTask")
}

func TestCustomTasksSkipped(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipelinerun-with-a-customtask", false, true)
	assert.NilError(t, err)
	assert.Equal(t, resolved.Spec.PipelineSpec.Tasks[0].Name, "shipwright")
	assert.Equal(t, string(resolved.Spec.PipelineSpec.Tasks[0].TaskRef.APIVersion), "shipwright.io/v1alpha1")
	assert.Equal(t, string(resolved.Spec.PipelineSpec.Tasks[0].TaskRef.Kind), "Build")
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

func TestPipelineRunWithFinallyV1(t *testing.T) {
	resolved, _, err := readTDfile(t, "pipeline-finally-v1", false, true)
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
	assert.Error(t, err, "could not find any PipelineRun in your .tekton/ directory")
}

func TestReferencedTaskNotInRepo(t *testing.T) {
	_, _, err := readTDfile(t, "referenced-task-not-in-repo", false, true)
	assert.Error(t, err, "cannot find referenced task nothere. if it's a remote task make sure to add it in the annotations")
}

func TestReferencedPipelineNotInRepo(t *testing.T) {
	_, _, err := readTDfile(t, "referenced-pipeline-not-in-repo", false, true)
	assert.Error(t, err, "cannot find referenced pipeline pipeline-test1. for a remote pipeline make sure to add it in the annotation")
}

func TestIgnoreDocSpace(t *testing.T) {
	_, _, err := readTDfile(t, "empty-spaces", false, true)
	assert.NilError(t, err)
}

func TestPipelineV1StayV1(t *testing.T) {
	got, _, err := readTDfile(t, "pipelinev1asv1", false, true)
	assert.NilError(t, err)
	assert.Equal(t, got.APIVersion, "tekton.dev/v1")
}

func TestPipelineRunv1Beta1InvalidConversion(t *testing.T) {
	t.Skip("Figure out the issue where setdefault sets the SA and fail when applying on osp")
	_, _, err := readTDfile(t, "pipelinerun-invalid-conversion", false, true)
	assert.ErrorContains(t, err, "cannot be validated")
}

func TestTaskv1Beta1InvalidConversion(t *testing.T) {
	t.Skip("Figure out the issue where setdefault sets the SA and fail when applying on osp")
	_, _, err := readTDfile(t, "task-invalid-conversion", false, true)
	assert.ErrorContains(t, err, "cannot be validated")
}

func TestPipelinev1Beta1InvalidConversion(t *testing.T) {
	t.Skip("Figure out the issue where setdefault sets the SA and fail when applying on osp")
	_, _, err := readTDfile(t, "pipeline-invalid-conversion", false, true)
	assert.ErrorContains(t, err, "cannot be validated")
}

func TestPipelineRunsWithSameName(t *testing.T) {
	tests := []struct {
		name string
		prs  []*tektonv1.PipelineRun
		err  string
	}{
		{
			name: "same name pipelineruns exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
					},
				},
			},
			err: "found multiple pipelinerun in .tekton with the same name: pipelinerun-abc, please update",
		},
		{
			name: "same name and generateName pipelineruns exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc",
					},
				},
			},
			err: "found multiple pipelinerun in .tekton with the same generateName: pipelinerun-abc, please update",
		},
		{
			name: "same generateName pipelineruns exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc-",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc-",
					},
				},
			},
			err: "found multiple pipelinerun in .tekton with the same generateName: pipelinerun-abc-, please update",
		},
		{
			name: "different pipelineruns exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc-",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-bcd",
					},
				},
			},
			err: "",
		},
		{
			name: "doesn't pipelinerun name exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-bcd",
					},
				},
			},
			err: "",
		},
		{
			name: "doesn't pipelinerun generateName exists",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-bcd",
					},
				},
			},
			err: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pipelineRunsWithSameName(tt.prs)
			if tt.err == "" {
				assert.NilError(t, err)
				return
			}
			assert.Equal(t, err.Error(), tt.err)
		})
	}
}

func TestMetadataResolve(t *testing.T) {
	tests := []struct {
		name    string
		prs     []*tektonv1.PipelineRun
		wantPRs []*tektonv1.PipelineRun
	}{
		{
			name: "label and annotation for pipelinerun name",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-deb",
					},
				},
			},
			wantPRs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-abc",
						Labels: map[string]string{
							apipac.OriginalPRName: "pipelinerun-abc",
						},
						Annotations: map[string]string{
							apipac.OriginalPRName: "pipelinerun-abc",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipelinerun-deb",
						Labels: map[string]string{
							apipac.OriginalPRName: "pipelinerun-deb",
						},
						Annotations: map[string]string{
							apipac.OriginalPRName: "pipelinerun-deb",
						},
					},
				},
			},
		},
		{
			name: "label and annotation for pipelinerun generateName",
			prs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc-",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-deb-",
					},
				},
			},
			wantPRs: []*tektonv1.PipelineRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-abc-",
						Labels: map[string]string{
							apipac.OriginalPRName: "pipelinerun-abc",
						},
						Annotations: map[string]string{
							apipac.OriginalPRName: "pipelinerun-abc-",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "pipelinerun-deb-",
						Labels: map[string]string{
							apipac.OriginalPRName: "pipelinerun-deb",
						},
						Annotations: map[string]string{
							apipac.OriginalPRName: "pipelinerun-deb-",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruns, err := MetadataResolve(tt.prs)
			assert.NilError(t, err)
			for i := range pruns {
				assert.DeepEqual(t, pruns[i], tt.wantPRs[i])
			}
		})
	}
}

func TestSkippingTask(t *testing.T) {
	skippedTasks := []string{"task1", "task3"}

	// Test case where taskName is in skippedTasks
	assert.Equal(t, skippingTask("task1", skippedTasks), true)

	// Test case where taskName is not in skippedTasks
	assert.Equal(t, skippingTask("task2", skippedTasks), false)
}
