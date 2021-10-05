package acl

import "testing"

func TestUserInOwnerFile(t *testing.T) {
	type args struct {
		ownerContent string
		sender       string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "user in owner file",
			args: args{
				ownerContent: "---\n approvers:\n  - allowed\n",
				sender:       "allowed",
			},
			want: true,
		},
		{
			name: "user not in owner file",
			args: args{
				ownerContent: "---\n approvers:\n  - allowed\n",
				sender:       "notallowed",
			},
			want: false,
		},
		{
			name: "bad yaml file",
			args: args{
				ownerContent: "bad",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserInOwnerFile(tt.args.ownerContent, tt.args.sender)
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

func TestMatchRegexp(t *testing.T) {
	type args struct {
		reg     string
		comment string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "match",
			args: args{
				reg:     ".*",
				comment: "hello",
			},
			want: true,
		},
		{
			name: "nomatch",
			args: args{
				reg:     "!!!",
				comment: "foobar",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchRegexp(tt.args.reg, tt.args.comment); got != tt.want {
				t.Errorf("MatchRegexp() = %v, want %v", got, tt.want)
			}
		})
	}
}
