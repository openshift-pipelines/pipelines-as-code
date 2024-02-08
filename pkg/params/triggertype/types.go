package triggertype

type (
	Trigger string
)

func (t Trigger) String() string {
	return string(t)
}

func StringToType(s string) Trigger {
	switch s {
	case OkToTest.String():
		return OkToTest
	case Retest.String():
		return Retest
	case Push.String():
		return Push
	case PullRequest.String():
		return PullRequest
	case Cancel.String():
		return Cancel
	case CheckSuiteRerequested.String():
		return CheckSuiteRerequested
	case CheckRunRerequested.String():
		return CheckRunRerequested
	case Incoming.String():
		return Incoming
	case Comment.String():
		return Comment
	}
	return ""
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
	Comment               Trigger = "comment"
)
