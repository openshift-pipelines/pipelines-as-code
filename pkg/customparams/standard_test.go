package customparams

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	testprovider "github.com/openshift-pipelines/pipelines-as-code/pkg/test/provider"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rectesting "knative.dev/pkg/reconciler/testing"
)

func TestMakeStandardParamsFromEvent(t *testing.T) {
	event := &info.Event{
		SHA:          "1234567890",
		Organization: "Org",
		Repository:   "Repo",
		BaseBranch:   "main",
		HeadBranch:   "foo",
		EventType:    "pull_request",
		Sender:       "SENDER",
		URL:          "https://paris.com",
		HeadURL:      "https://india.com",
	}

	result := map[string]string{
		"event_type":        "pull_request",
		"repo_name":         "repo",
		"repo_owner":        "org",
		"repo_url":          "https://paris.com",
		"source_url":        "https://india.com",
		"revision":          "1234567890",
		"sender":            "sender",
		"source_branch":     "foo",
		"target_branch":     "main",
		"target_namespace":  "myns",
		"all_changed_files": "added.go,deleted.go,modified.go,renamed.go",
		"added_files":       "added.go",
		"deleted_files":     "deleted.go",
		"modified_files":    "modified.go",
		"renamed_files":     "renamed.go",
	}

	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
	}

	ctx, _ := rectesting.SetupFakeContext(t)
	vcx := &testprovider.TestProviderImp{
		WantAllChangedFiles: []string{"added.go", "deleted.go", "modified.go", "renamed.go"},
		WantAddedFiles:      []string{"added.go"},
		WantDeletedFiles:    []string{"deleted.go"},
		WantModifiedFiles:   []string{"modified.go"},
		WantRenamedFiles:    []string{"renamed.go"},
	}

	p := NewCustomParams(event, repo, nil, nil, nil, vcx)
	params := p.makeStandardParamsFromEvent(ctx)
	assert.DeepEqual(t, params, result)

	nevent := &info.Event{}
	event.DeepCopyInto(nevent)
	nevent.CloneURL = "https://blahblah"
	p.event = nevent
	nparams := p.makeStandardParamsFromEvent(ctx)
	result["repo_url"] = nevent.CloneURL
	assert.DeepEqual(t, nparams, result)
}
