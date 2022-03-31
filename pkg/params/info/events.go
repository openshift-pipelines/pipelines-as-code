package info

import "net/http"

type Event struct {
	Event interface{}

	// EventType is what coming from the provider header, i.e:
	// GitHub -> pull_request
	// GitLab -> Merge Request Hook
	// Usually used for payload filtering passed from trigger directly
	EventType string

	// Full request
	Request *Request

	// TriggerTarget stable field across providers, ie: on Gitlab, Github and
	// others it would be always be pull_request we can rely on to know if it's
	// a push or a pull_request
	TriggerTarget string

	BaseBranch    string // branch against where we are making the PR
	DefaultBranch string // master/main branches to know where things like the OWNERS file is located.
	HeadBranch    string // branch from where our SHA get tested
	SHA           string
	Sender        string
	URL           string // WEB url not the git URL, which would match to the repo.spec
	SHAURL        string // pretty URL for web browsing for UIs (cli/web)
	SHATitle      string // commit title for UIs

	// TODO: move forge specifics to each driver
	// Github
	Organization string
	Repository   string
	CheckRunID   *int64

	// Bitbucket Cloud
	AccountID string

	// Bitbucket Server
	CloneURL string // bitbucket server has a different url for cloning the repo than normal public html url
	Provider *Provider
}

type Provider struct {
	Token                 string
	URL                   string
	User                  string
	InfoFromRepo          bool // whether the provider info come from the repository
	WebhookSecret         string
	WebhookSecretFromRepo bool
}

type Request struct {
	Header  http.Header
	Payload []byte
}

// DeepCopyInto deep copy runinfo in another instance
func (r *Event) DeepCopyInto(out *Event) {
	*out = *r
}

// NewEvent returns a new Event
func NewEvent() *Event {
	return &Event{
		Provider: &Provider{},
	}
}
