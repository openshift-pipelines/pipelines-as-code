package info

import (
	"net/http"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
)

type Event struct {
	State
	Event interface{}

	// EventType is what coming from the provider header, i.e:
	// GitHub -> pull_request
	// GitLab -> Merge Request Hook
	// Incoming Webhook  -> incoming (always a push)
	// Usually used for payload filtering passed from trigger directly
	EventType string

	// Full request
	Request *Request

	// TriggerTarget stable field across providers, ie: on Gitlab, Github and
	// others it would be always be pull_request we can rely on to know if it's
	// a push or a pull_request
	TriggerTarget triggertype.Trigger

	// Target PipelineRun, the target PipelineRun user request. Used in incoming webhook
	TargetPipelineRun string

	BaseBranch    string // branch against where we are making the PR
	DefaultBranch string // master/main branches to know where things like the OWNERS file is located.
	HeadBranch    string // branch from where our SHA get tested
	BaseURL       string // url against where we are making the PR
	HeadURL       string // url from where our SHA get tested
	SHA           string
	Sender        string
	URL           string // WEB url not the git URL, which would match to the repo.spec
	SHAURL        string // pretty URL for web browsing for UIs (cli/web)
	SHATitle      string // commit title for UIs

	PullRequestNumber int      // Pull or Merge Request number
	PullRequestTitle  string   // Title of the pull Request
	PullRequestLabel  []string // Labels of the pull Request
	TriggerComment    string   // The comment triggering the pipelinerun when using on-comment annotation

	// TODO: move forge specifics to each driver
	// Github
	Organization   string
	Repository     string
	InstallationID int64
	GHEURL         string

	// TODO: move out inside the provider
	// Bitbucket Cloud
	AccountID string

	// TODO: move out inside the provider
	// Bitbucket Data Center
	CloneURL string // bitbucket data center has a different url for cloning the repo than normal public html url
	Provider *Provider

	// Gitlab
	SourceProjectID int
	TargetProjectID int
}

type State struct {
	TargetTestPipelineRun   string
	CancelPipelineRuns      bool
	TargetCancelPipelineRun string
}

type Provider struct {
	Token                 string
	URL                   string
	User                  string
	WebhookSecret         string
	WebhookSecretFromRepo bool
}

type Request struct {
	Header  http.Header
	Payload []byte
}

// DeepCopyInto deep copy runinfo in another instance.
func (r *Event) DeepCopyInto(out *Event) {
	*out = *r
}

// NewEvent returns a new Event.
func NewEvent() *Event {
	return &Event{
		Provider: &Provider{},
		Request:  &Request{},
	}
}
