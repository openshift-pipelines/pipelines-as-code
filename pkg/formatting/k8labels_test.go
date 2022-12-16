package formatting

import "testing"

func TestK8LabelsCleanup(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{
		{
			name: "clean characters for k8 labels",
			str:  "foo/bar hello",
			want: "foo-bar_hello",
		},
		{
			name: "keep dash",
			str:  "foo-bar_hello",
			want: "foo-bar_hello",
		},
		{
			name: "github bot name",
			str:  "github-actions[bot]",
			want: "github-actions__bot",
		},
		{
			name: "trailing dash name removed",
			str:  "MBAPPEvsMESSI--",
			want: "MBAPPEvsMESSI",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := K8LabelsCleanup(tt.str); got != tt.want {
				t.Errorf("K8LabelsCleanup() = %v, want %v", got, tt.want)
			}
		})
	}
}
