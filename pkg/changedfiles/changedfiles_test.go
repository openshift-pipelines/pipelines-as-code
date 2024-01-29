package changedfiles

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		expected []string
	}{
		{
			name:     "no duplicates",
			initial:  []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			initial:  []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDuplicates(tt.initial)
			assert.DeepEqual(t, got, tt.expected)
		})
	}
}

func TestRemoveDuplicatesMethod(t *testing.T) {
	tests := []struct {
		name     string
		initial  ChangedFiles
		expected ChangedFiles
	}{
		{
			name: "no duplicates",
			initial: ChangedFiles{
				All:      []string{"a", "b", "c"},
				Added:    []string{"a", "b", "c"},
				Deleted:  []string{"a", "b", "c"},
				Modified: []string{"a", "b", "c"},
				Renamed:  []string{"a", "b", "c"},
			},
			expected: ChangedFiles{
				All:      []string{"a", "b", "c"},
				Added:    []string{"a", "b", "c"},
				Deleted:  []string{"a", "b", "c"},
				Modified: []string{"a", "b", "c"},
				Renamed:  []string{"a", "b", "c"},
			},
		},
		{
			name: "with duplicates",
			initial: ChangedFiles{
				All:      []string{"a", "b", "a", "c", "b"},
				Added:    []string{"a", "b", "a", "c", "b"},
				Deleted:  []string{"a", "b", "a", "c", "b"},
				Modified: []string{"a", "b", "a", "c", "b"},
				Renamed:  []string{"a", "b", "a", "c", "b"},
			},
			expected: ChangedFiles{
				All:      []string{"a", "b", "c"},
				Added:    []string{"a", "b", "c"},
				Deleted:  []string{"a", "b", "c"},
				Modified: []string{"a", "b", "c"},
				Renamed:  []string{"a", "b", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.RemoveDuplicates()
			assert.DeepEqual(t, tt.initial, tt.expected)
		})
	}
}
