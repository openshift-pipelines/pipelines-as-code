package resolve

import (
	"fmt"
	"strings"
	"testing"

	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/matcher"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/clients"
	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	ttkn "github.com/openshift-pipelines/pipelines-as-code/pkg/test/tekton"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
	"sigs.k8s.io/yaml"
)

func TestRemote(t *testing.T) {
	randomPipelineRunName := "pipelinerun-abc"
	remotePipelineName := "remote-pipeline"
	remotePipelineURL := "http://remote/" + remotePipelineName

	taskFromPipelineRunName := "task-from-pipelinerun"
	taskFromPipelineRunURL := "http://remote/" + taskFromPipelineRunName

	remoteTaskName := "remote-task"
	remoteTaskURL := "http://remote/" + remoteTaskName
	taskFromPipelineSpec := tektonv1.TaskSpec{
		Steps: []tektonv1.Step{
			{
				Name:    "step1",
				Image:   "scratch",
				Command: []string{"true"},
			},
		},
	}
	taskFromPipelineRunSpec := tektonv1.TaskSpec{
		Steps: []tektonv1.Step{
			{
				Name:    "frompipelinerun",
				Image:   "scratch",
				Command: []string{"false"},
			},
		},
	}
	pipelineTaskEmbedded := []tektonv1.PipelineTask{
		{
			Name: "embedded",
			TaskSpec: &tektonv1.EmbeddedTask{
				TaskSpec: taskFromPipelineSpec,
			},
		},
	}
	pipelinewithTaskEmbedded := ttkn.MakePipeline(remotePipelineName, pipelineTaskEmbedded, nil)
	pipelinewithTaskEmbeddedB, err := yaml.Marshal(pipelinewithTaskEmbedded)
	assert.NilError(t, err)

	pipelineTaskRef := []tektonv1.PipelineTask{
		{
			Name: remoteTaskName,
		},
	}
	pipelinewithTaskRef := ttkn.MakePipeline(remotePipelineName, pipelineTaskRef, map[string]string{
		apipac.Task: remoteTaskURL,
	})
	pipelinewithTaskRefYamlB, err := yaml.Marshal(pipelinewithTaskRef)
	assert.NilError(t, err)

	singleTask := ttkn.MakeTask(remoteTaskName, taskFromPipelineSpec)
	singleTaskB, err := yaml.Marshal(singleTask)
	assert.NilError(t, err)

	taskFromPipelineRun := ttkn.MakeTask(remoteTaskName, taskFromPipelineRunSpec)
	taskFromPipelineRunB, err := yaml.Marshal(taskFromPipelineRun)
	assert.NilError(t, err)

	tests := []struct {
		name                   string
		pipelineruns           []*tektonv1.PipelineRun
		tasks                  []*tektonv1.Task
		pipelines              []*tektonv1.Pipeline
		wantErrSnippet         string
		remoteURLS             map[string]map[string]string
		expectedLogsSnippets   []string
		expectedTaskSpec       tektonv1.TaskSpec
		expectedPipelinesFetch int
		expectedTaskFetch      int
	}{
		{
			name: "remote pipeline with remote task from pipeline",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline: remotePipelineURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
			remoteURLS: map[string]map[string]string{
				"http://remote/embedpipeline": {
					"body": string(pipelinewithTaskEmbeddedB),
					"code": "200",
				},
				remotePipelineURL: {
					"body": string(pipelinewithTaskRefYamlB),
					"code": "200",
				},
				remoteTaskURL: {
					"body": string(singleTaskB),
					"code": "200",
				},
			},
			expectedTaskSpec: taskFromPipelineSpec,
			expectedLogsSnippets: []string{
				fmt.Sprintf("successfully fetched %s from remote https url", remotePipelineURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
			},
			expectedPipelinesFetch: 1,
			expectedTaskFetch:      1,
		},
		{
			name: "remote pipeline with remote task in pipeline overridden from pipelinerun",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline: remotePipelineURL,
					apipac.Task:     taskFromPipelineRunURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
			expectedTaskSpec: taskFromPipelineRunSpec,
			remoteURLS: map[string]map[string]string{
				remotePipelineURL: {
					"body": string(pipelinewithTaskRefYamlB),
					"code": "200",
				},
				remoteTaskURL: {
					"body": string(singleTaskB),
					"code": "200",
				},
				taskFromPipelineRunURL: {
					"body": string(taskFromPipelineRunB),
					"code": "200",
				},
			},
			expectedLogsSnippets: []string{
				fmt.Sprintf("successfully fetched %s from remote https url", taskFromPipelineRunURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remotePipelineURL),
			},
			expectedPipelinesFetch: 1,
			expectedTaskFetch:      1,
		},
		{
			name: "remote pipelinerun no annotations",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
		},
		{
			name:           "error/remote pipelinerun is 404",
			wantErrSnippet: "error getting remote pipeline " + remotePipelineURL,
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline: remotePipelineURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
		},
		{
			name: "skipping/multiple tasks of the same name from pipelinerun annotations and pipeline annotation",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline:    remotePipelineURL,
					apipac.Task:        remoteTaskURL,
					apipac.Task + "-1": remoteTaskURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
			remoteURLS: map[string]map[string]string{
				remotePipelineURL: {
					"body": string(pipelinewithTaskRefYamlB),
					"code": "200",
				},
				remoteTaskURL: {
					"body": string(singleTaskB),
					"code": "200",
				},
			},
			expectedTaskSpec: taskFromPipelineSpec,
			expectedLogsSnippets: []string{
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remotePipelineURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
			},
			expectedPipelinesFetch: 1,
			expectedTaskFetch:      1,
		},
		{
			name: "skipping/multiple tasks of the same name from pipelinerun annotations and tektondir",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline: remotePipelineURL,
					apipac.Task:     remoteTaskURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
			tasks: []*tektonv1.Task{
				singleTask,
			},
			remoteURLS: map[string]map[string]string{
				remotePipelineURL: {
					"body": string(pipelinewithTaskRefYamlB),
					"code": "200",
				},
				remoteTaskURL: {
					"body": string(singleTaskB),
					"code": "200",
				},
			},
			expectedTaskSpec: taskFromPipelineSpec,
			expectedLogsSnippets: []string{
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remotePipelineURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("skipping remote task %s from remote pipeline %s as already defined in pipelinerun", remoteTaskName, remotePipelineName),
				fmt.Sprintf("overriding task %s coming from tekton directory by an annotation task on the pipeline or pipelinerun", remoteTaskName),
			},
			expectedPipelinesFetch: 1,
			expectedTaskFetch:      1,
		},
		{
			name: "skipping/multiple pipelines of the same name from pipelinerun annotations and tektondir",
			pipelineruns: []*tektonv1.PipelineRun{
				ttkn.MakePR(randomPipelineRunName, map[string]string{
					apipac.Pipeline: remotePipelineURL,
					apipac.Task:     remoteTaskURL,
				},
					tektonv1.PipelineRunSpec{
						PipelineRef: &tektonv1.PipelineRef{
							Name: "remote-pipeline",
						},
					},
				),
			},
			pipelines: []*tektonv1.Pipeline{
				pipelinewithTaskEmbedded,
			},
			remoteURLS: map[string]map[string]string{
				remotePipelineURL: {
					"body": string(pipelinewithTaskRefYamlB),
					"code": "200",
				},
				remoteTaskURL: {
					"body": string(singleTaskB),
					"code": "200",
				},
			},
			expectedTaskSpec: taskFromPipelineSpec,
			expectedLogsSnippets: []string{
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remotePipelineURL),
				fmt.Sprintf("successfully fetched %s from remote https url", remoteTaskURL),
				fmt.Sprintf("skipping remote task %s from remote pipeline %s as already defined in pipelinerun", remoteTaskName, remotePipelineName),
				fmt.Sprintf("overriding pipeline %s coming from tekton directory by the annotation pipelinerun", remotePipelineName),
			},
			expectedPipelinesFetch: 1,
			expectedTaskFetch:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer, log := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			ctx, _ := rtesting.SetupFakeContext(t)
			tktype := TektonTypes{
				Pipelines:    tt.pipelines,
				Tasks:        tt.tasks,
				PipelineRuns: tt.pipelineruns,
			}

			tprovider := &testprovider.TestProviderImp{}
			httpTestClient := httptesthelper.MakeHTTPTestClient(tt.remoteURLS)
			rt := &matcher.RemoteTasks{
				ProviderInterface: tprovider,
				Logger:            logger,
				Run: &params.Run{
					Clients: clients.Clients{
						HTTP: *httpTestClient,
					},
				},
			}
			ret, err := getRemotes(ctx, rt, tktype)
			if tt.wantErrSnippet != "" {
				assert.ErrorContains(t, err, tt.wantErrSnippet)
				return
			}
			assert.NilError(t, err)

			allPipelinesNames := []string{}
			for _, task := range ret.Pipelines {
				allPipelinesNames = append(allPipelinesNames, task.GetName())
			}
			assert.Equal(t, len(ret.Pipelines), tt.expectedPipelinesFetch, allPipelinesNames)

			allTasksNames := []string{}
			for _, task := range ret.Tasks {
				allTasksNames = append(allTasksNames, task.GetName())
			}
			assert.Equal(t, len(ret.Tasks), tt.expectedTaskFetch, allTasksNames)

			for k, snippet := range tt.expectedLogsSnippets {
				logmsg := log.AllUntimed()[k].Message
				assert.Assert(t, strings.Contains(logmsg, snippet), "\n on index: %d\n we want: %s\n we  got: %s", k, snippet, logmsg)
			}
			if tt.expectedTaskFetch > 0 {
				assert.DeepEqual(t, tt.expectedTaskSpec, ret.Tasks[0].Spec)
			}
		})
	}
}
