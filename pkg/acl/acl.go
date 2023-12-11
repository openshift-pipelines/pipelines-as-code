package acl

import (
	"regexp"

	"sigs.k8s.io/yaml"
)

const OKToTestCommentRegexp = `(^|\n)\/ok-to-test(\r\n|\r|\n|$)`

// ownersConfig prow owner, only supporting approvers or reviewers in yaml.
type ownersConfig struct {
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// UserInOwnerFile Parse a Prow type Owner, Approver files and return true if the sender is in
// there.
// Does not support OWNERS_ALIASES.
func UserInOwnerFile(ownerContent, sender string) (bool, error) {
	oc := ownersConfig{}
	err := yaml.Unmarshal([]byte(ownerContent), &oc)
	if err != nil {
		return false, err
	}

	for _, owner := range append(oc.Approvers, oc.Reviewers...) {
		if owner == sender {
			return true, nil
		}
	}
	return false, nil
}

// MatchRegexp Match a regexp to a string.
func MatchRegexp(reg, comment string) bool {
	re := regexp.MustCompile(reg)
	return string(re.Find([]byte(comment))) != ""
}
