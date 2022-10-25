package formatting

import (
	"testing"
)

func TestCamelCasit(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "pull_request",
			args: args{s: "pull_request"},
			want: "PullRequest",
		},
		{
			name: "oneword",
			args: args{s: "push"},
			want: "Push",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CamelCasit(tt.args.s); got != tt.want {
				t.Errorf("CamelCasit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRepoOwnerSplitted(t *testing.T) {
	tests := []struct {
		name    string
		retOrg  string
		retRepo string
		wantErr bool
		url     string
	}{
		{
			name:    "good/parse url",
			url:     "https://forge/owner/repo",
			retOrg:  "owner",
			retRepo: "repo",
		},
		{
			name:    "good/parse url gitlab subpath",
			url:     "https://forge/foo/bar/owner/repo",
			retOrg:  "foo/bar/owner",
			retRepo: "repo",
		},
		{
			name:    "bad/no org/repo in url",
			url:     "https://forge/repo",
			wantErr: true,
		},
		{
			name:    "bad/url",
			url:     "hello/repo",
			wantErr: true,
		},
		{
			name:    "bad/chars in url",
			url:     "😃",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetRepoOwnerSplitted(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("getOrgRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.retOrg {
				t.Errorf("getOrgRepoURL() got = %v, retOrg %v", got, tt.retOrg)
			}
			if got1 != tt.retRepo {
				t.Errorf("getOrgRepoURL() got1 = %v, retOrg %v", got1, tt.retRepo)
			}
		})
	}
}

func TestGetRepoOwnerFromGHURL(t *testing.T) {
	type args struct {
		ghURL string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "repoowner",
			args: args{
				ghURL: "https://allo/hello/moto",
			},
			want:    "hello/moto",
			wantErr: false,
		},
		{
			name: "repoowner with capital letters",
			args: args{
				ghURL: "https://allo/HELLO/moto",
			},
			want:    "hello/moto",
			wantErr: false,
		},
		{
			name: "bad url",
			args: args{
				ghURL: "xx",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRepoOwnerFromURL(tt.args.ghURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRepoOwnerFromGHURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetRepoOwnerFromGHURL() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeBranch(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "sanitize branch",
			args: args{s: "refs/heads/foo"},
			want: "foo",
		},
		{
			name: "don't sanitize tags",
			args: args{s: "refs/tags/1.0"},
			want: "refs/tags/1.0",
		},
		{
			name: "sanitize main ref",
			args: args{s: "refs-heads-main"},
			want: "main",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeBranch(tt.args.s); got != tt.want {
				t.Errorf("SanitizeBranch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	type args struct {
		sha string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "shorten sha",
			args: args{
				sha: "1234567890",
			},
			want: "1234567",
		},
		{
			name: "nada",
			args: args{
				sha: "",
			},
			want: "",
		},
		{
			name: "very short",
			args: args{
				sha: "123",
			},
			want: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortSHA(tt.args.sha); got != tt.want {
				t.Errorf("ShortSHA() = %v, want %v", got, tt.want)
			}
		})
	}
}
