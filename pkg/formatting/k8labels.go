package formatting

import "strings"

// CleanValueKubernetes k8s do not like slash in labels value and on push we have the
// full ref, we replace the "/" by "-". The tools probably need to be aware of
// it when querying.
//
// valid label must be an empty string or consist of alphanumeric characters,
// '-', '_' or '.', and must start and end with an alphanumeric character
// (e.g. 'MyValue', or 'my_value', or '12345', regex used for validation is
// '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?').
func CleanValueKubernetes(s string) string {
	replasoeur := strings.NewReplacer("/", "-", " ", "_", "[", "__", "]", "__")
	replaced := replasoeur.Replace(strings.TrimRight(s, " -_[]"))
	return strings.TrimSpace(replaced)
}
