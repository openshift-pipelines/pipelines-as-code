package acl

import (
	"testing"

	"golang.org/x/exp/slices"
)

func TestUserInOwnerFile(t *testing.T) {
	type args struct {
		ownersContent        string
		ownersAliasesContent string
		sender               string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "user in approvers",
			args: args{
				ownersContent:        "---\n approvers:\n  - allowed\n",
				ownersAliasesContent: "",
				sender:               "allowed",
			},
			want: true,
		},
		{
			name: "user in reviewers",
			args: args{
				ownersContent:        "---\n reviewers:\n  - allowed\n",
				ownersAliasesContent: "",
				sender:               "allowed",
			},
			want: true,
		},
		{
			name: "user not in owner file",
			args: args{
				ownersContent:        "---\n approvers:\n  - allowed\n",
				ownersAliasesContent: "",
				sender:               "notallowed",
			},
			want: false,
		},
		{
			name: "user in owners_aliases file",
			args: args{
				ownersContent:        "---\n approvers:\n  - allowed-alias\n",
				ownersAliasesContent: "---\n aliases:\n  allowed-alias:\n  - allowed",
				sender:               "allowed",
			},
			want: true,
		},
		{
			name: "user not in owners_aliases file",
			args: args{
				ownersContent:        "---\n approvers:\n  - allowed-alias\n",
				ownersAliasesContent: "---\n aliases:\n  allowed-alias:\n  - allowed",
				sender:               "notallowed",
			},
			want: false,
		},
		{
			name: "user in .* filters",
			args: args{
				ownersContent:        "---\n filters:\n  .*:\n    approvers:\n    - allowed",
				ownersAliasesContent: "",
				sender:               "allowed",
			},
			want: true,
		},
		{
			name: "user not in .* filters",
			args: args{
				ownersContent:        "---\n filters:\n  .*:\n    approvers:\n    - allowed",
				ownersAliasesContent: "",
				sender:               "notallowed",
			},
			want: false,
		},
		{
			name: "user in other filters",
			args: args{
				ownersContent:        "---\n filters:\n  somefilter:\n    approvers:\n    - allowed",
				ownersAliasesContent: "",
				sender:               "allowed",
			},
			want: false,
		},
		{
			name: "user alias in .* filters",
			args: args{
				ownersContent:        "---\n filters:\n  .*:\n    approvers:\n    - allowed",
				ownersAliasesContent: "---\n aliases:\n  allowed-alias:\n  - allowed",
				sender:               "allowed",
			},
			want: true,
		},
		{
			name: "no owners file",
			args: args{
				ownersContent:        "",
				ownersAliasesContent: "",
			},
			want: false,
		},
		{
			name: "bad owners yaml file",
			args: args{
				ownersContent:        "bad",
				ownersAliasesContent: "",
			},
			wantErr: true,
		},
		{
			name: "bad owners_aliases yaml file",
			args: args{
				ownersContent:        "",
				ownersAliasesContent: "bad",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserInOwnerFile(tt.args.ownersContent, tt.args.ownersAliasesContent, tt.args.sender)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserInOwnerFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("UserInOwnerFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandAliases(t *testing.T) {
	type args struct {
		owners  []string
		aliases aliases
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no owner or aliases",
			args: args{
				owners:  []string{},
				aliases: map[string][]string{},
			},
			want: []string{},
		},
		{
			name: "no owner have aliases",
			args: args{
				owners:  []string{},
				aliases: map[string][]string{"alias": {"foo"}},
			},
			want: []string{},
		},
		{
			name: "owners dedups",
			args: args{
				owners:  []string{"foo", "foo"},
				aliases: map[string][]string{},
			},
			want: []string{"foo"},
		},
		{
			name: "expand alias",
			args: args{
				owners:  []string{"foo", "aliasA", "aliasB"},
				aliases: map[string][]string{"aliasA": {"bar"}, "aliasB": {"baz"}},
			},
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "expand alias dedups",
			args: args{
				owners:  []string{"foo", "aliasA", "aliasB"},
				aliases: map[string][]string{"aliasA": {"foo"}, "aliasB": {"bar"}},
			},
			want: []string{"foo", "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandAliases(tt.args.owners, tt.args.aliases)
			// Can't use reflect.DeepEqual to compare the slices as expandAliases
			// uses a map for dedup which does not preserve the order.
			// We do not care about the order, just the content of the slice.
			if len(got) != len(tt.want) {
				t.Errorf("expandAliases() got = %v, want %v", got, tt.want)
			}
			for _, v := range got {
				if !slices.Contains(tt.want, v) {
					t.Errorf("expandAliases() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
