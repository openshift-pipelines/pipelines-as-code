package triggertype

type (
	Trigger string
)

func (t Trigger) String() string {
	return string(t)
}

const (
	OkToTest              Trigger = "ok-to-test"
	Retest                Trigger = "retest"
	Push                  Trigger = "push"
	PullRequest           Trigger = "pull_request"
	Cancel                Trigger = "cancel"
	CheckSuiteRerequested Trigger = "check-suite-rerequested"
	CheckRunRerequested   Trigger = "check-run-rerequested"
	Incoming              Trigger = "incoming"
)
