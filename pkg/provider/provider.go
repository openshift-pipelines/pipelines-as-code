package provider

const (
	RetestRegex   = "(^|\\\\r\\\\n)/retest([ ]*$|$|\\\\r\\\\n)"
	OktotestRegex = "(^|\\\\r\\\\n)/ok-to-test([ ]*$|$|\\\\r\\\\n)"
)

func Valid(value string, validValues []string) bool {
	for _, v := range validValues {
		if v == value {
			return true
		}
	}
	return false
}
