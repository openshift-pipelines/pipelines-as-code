package changedfiles

type ChangedFiles struct {
	All      []string
	Added    []string
	Deleted  []string
	Modified []string
	Renamed  []string
}

// removeDuplicates removes duplicates from a slice of strings.
func removeDuplicates(s []string) []string {
	holdit := make(map[string]struct{})
	result := make([]string, 0, len(s))
	for _, str := range s {
		if _, ok := holdit[str]; !ok {
			holdit[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

func (c *ChangedFiles) RemoveDuplicates() {
	c.All = removeDuplicates(c.All)
	c.Added = removeDuplicates(c.Added)
	c.Deleted = removeDuplicates(c.Deleted)
	c.Modified = removeDuplicates(c.Modified)
	c.Renamed = removeDuplicates(c.Renamed)
}
