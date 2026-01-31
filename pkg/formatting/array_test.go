package formatting

import (
	"reflect"
	"testing"
)

func TestUniqueStringArray(t *testing.T) {
	type args struct {
		slice []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no duplicates",
			args: args{
				slice: []string{"1", "2"},
			},
			want: []string{"1", "2"},
		},

		{
			name: "with duplicates",
			args: args{
				slice: []string{"1", "2", "1", "2", "1", "2", "3", "2", "5", "5"},
			},
			want: []string{"1", "2", "3", "5"},
		},
		{
			name: "empty slice",
			args: args{
				slice: []string{},
			},
			want: []string{},
		},
		{
			name: "single element",
			args: args{
				slice: []string{"only"},
			},
			want: []string{"only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UniqueStringArray(tt.args.slice); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UniqueStringArray() = %v, want %v", got, tt.want)
			}
		})
	}
}
