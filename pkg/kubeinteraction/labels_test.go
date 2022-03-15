package kubeinteraction

import (
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddLabelsAndAnnotations(t *testing.T) {
	type args struct {
		event       *info.Event
		pipelineRun *tektonv1beta1.PipelineRun
		repo        *apipac.Repository
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test label and annotation added to pr",
			args: args{
				event: &info.Event{
					Organization: "org",
					Repository:   "repo",
					SHA:          "sha",
					Sender:       "sender",
					EventType:    "pull_request",
					BaseBranch:   "main",
					SHAURL:       "https://url/sha",
				},
				pipelineRun: &tektonv1beta1.PipelineRun{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{},
						Annotations: map[string]string{},
					},
				},
				repo: &apipac.Repository{
					ObjectMeta: metav1.ObjectMeta{
						Name: "repo",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddLabelsAndAnnotations(tt.args.event, tt.args.pipelineRun, tt.args.repo)
			assert.Assert(t, tt.args.pipelineRun.Labels[filepath.Join(pipelinesascode.GroupName, "url-org")] == tt.args.event.Organization, "'%s' != %s",
				tt.args.pipelineRun.Labels[filepath.Join(pipelinesascode.GroupName, "url-org")], tt.args.event.Organization)
			assert.Assert(t, tt.args.pipelineRun.Annotations[filepath.Join(pipelinesascode.GroupName,
				"sha-url")] == tt.args.event.SHAURL)
		})
	}
}
