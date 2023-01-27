package templates

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReplacePlaceHoldersVariables(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
		dicto    map[string]string
	}{
		{
			name:     "Test Replace",
			template: `revision: {{ revision }}} url: {{ url }} bar: {{ bar}}`,
			expected: `revision: master} url: https://chmouel.com bar: {{ bar}}`,
			dicto: map[string]string{
				"revision": "master",
				"url":      "https://chmouel.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplacePlaceHoldersVariables(tt.template, tt.dicto)
			if d := cmp.Diff(got, tt.expected); d != "" {
				t.Fatalf("-got, +want: %v", d)
			}
		})
	}
}

func TestProcessTemplates(t *testing.T) {
	tests := []struct {
		name       string
		event      *info.Event
		template   string
		expected   string
		repository *v1alpha1.Repository
	}{
		{
			name: "test process templates",
			event: &info.Event{
				SHA:          "abcd",
				URL:          "http://chmouel.com",
				Organization: "owner",
				Repository:   "repository",
				HeadBranch:   "ohyeah",
				BaseBranch:   "ohno",
				Sender:       "apollo",
			},
			template: `{{ revision }} {{ repo_owner }} {{ repo_name }} {{ repo_url }} {{ source_branch }} {{ target_branch }} {{ sender }}`,
			expected: "abcd owner repository http://chmouel.com ohyeah ohno apollo",
		},
		{
			name: "strip refs head from branches",
			event: &info.Event{
				HeadBranch: "refs/heads/ohyeah",
				BaseBranch: "refs/heads/ohno",
				Sender:     "apollo",
			},
			template: `{{ source_branch }} {{ target_branch }}`,
			expected: "ohyeah ohno",
		},
		{
			name: "process pull request number",
			event: &info.Event{
				PullRequestNumber: 666,
			},
			template: `{{ pull_request_number }}`,
			expected: "666",
		},
		{
			name:     "no pull request no nothing",
			event:    &info.Event{},
			template: `{{ pull_request_number }}`,
			expected: `{{ pull_request_number }}`,
		},
		{
			name: "test process templates lowering owner and repository",
			event: &info.Event{
				Organization: "OWNER",
				Repository:   "REPOSITORY",
			},
			template: `{{ repo_owner }} {{ repo_name }}`,
			expected: "owner repository",
		},
		{
			name: "test process use cloneurl",
			event: &info.Event{
				CloneURL: "https://cloneurl",
				URL:      "http://chmouel.com",
			},
			template: `{{ repo_url }}`,
			expected: "https://cloneurl",
		},
		{
			name: "replace target_namespace",
			event: &info.Event{
				CloneURL: "https://cloneurl",
				URL:      "http://chmouel.com",
			},
			template: `Install in {{ target_namespace }}`,
			expected: "Install in the_namespace",
			repository: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "the_namespace",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.repository
			if repo == nil {
				repo = &v1alpha1.Repository{}
			}
			processed := Process(tt.event, repo, tt.template)
			assert.Equal(t, tt.expected, processed)
		})
	}
}
