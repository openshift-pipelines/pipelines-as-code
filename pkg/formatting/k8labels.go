package formatting

import "strings"

// K8LabelsCleanup k8s do not like slash in labels value and on push we have the
// full ref, we replace the "/" by "-". The tools probably need to be aware of
// it when querying.
func K8LabelsCleanup(s string) string {
	replasoeur := strings.NewReplacer("/", "-", " ", "_", "[", "__", "]", "__")
	return replasoeur.Replace(s)
}
