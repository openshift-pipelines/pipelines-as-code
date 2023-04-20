package customparams

import (
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}

	result := map[string]string{
		"event_type":       "pull_request",
		"repo_name":        "repo",
		"repo_owner":       "org",
		"repo_url":         "https://paris.com",
		"revision":         "1234567890",
		"sender":           "sender",
		"source_branch":    "foo",
		"target_branch":    "main",
		"target_namespace": "myns",
	}

	repo := &v1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
	}

	p := NewCustomParams(event, repo, nil, nil, nil)
	params := p.makeStandardParamsFromEvent()
	assert.DeepEqual(t, params, result)

	nevent := &info.Event{}
	event.DeepCopyInto(nevent)
	nevent.CloneURL = "https://blahblah"
	p.event = nevent
	nparams := p.makeStandardParamsFromEvent()
	result["repo_url"] = nevent.CloneURL
	assert.DeepEqual(t, nparams, result)
}
