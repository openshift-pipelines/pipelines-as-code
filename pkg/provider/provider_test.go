package provider

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCompareHostOfURLS(t *testing.T) {
	tests := []struct {
		name string
		url1 string
		url2 string
		want bool
	}{
		{
			name: "exact same",
			url1: "https://shivam.com/foo/bar",
			url2: "https://shivam.com/hello/moto",
			want: true,
		},
		{
			name: "exact same but different",
			url1: "https://shivam.com/foo/bar",
			url2: "https://vincent.com/foo/bar",
			want: false,
		},
		{
			name: "bad url1",
			url1: "i am such a bad url",
			want: false,
		},
		{
			name: "bad url2",
			url2: "i am the baddest, choose me!",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareHostOfURLS(tt.url1, tt.url2)
			assert.Equal(t, tt.want, got)
		})
	}
}
