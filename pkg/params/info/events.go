package info

type Event struct {
	Event         interface{}
	EventType     string
	TriggerTarget string

	BaseBranch    string // branch against where we are making the PR
	DefaultBranch string
	HeadBranch    string // branch from where our SHA get tested
	Organization  string
	Repository    string
	SHA           string
	SHAURL        string
	Sender        string
	URL           string
	SHATitle      string

	// TODO: move forge specifics to each driver
	// Github
	CheckRunID *int64

	// Bitbucket
	AccountID string

	// Gitlab
	ProjectID int

	// Bitbucket Server
	CloneURL string // bitbucket server has a different cloneurl than normal url
}

// DeepCopyInto deep copy runinfo in another instance
func (r *Event) DeepCopyInto(out *Event) {
	*out = *r
}
