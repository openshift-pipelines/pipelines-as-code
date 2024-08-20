package context

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cli/cli/v2/git"
	"github.com/cli/cli/v2/internal/ghrepo"
)

// Remotes represents a set of git remotes
type Remotes []*Remote

// FindByName returns the first Remote whose name matches the list
func (r Remotes) FindByName(names ...string) (*Remote, error) {
	for _, name := range names {
		for _, rem := range r {
			if rem.Name == name || name == "*" {
				return rem, nil
			}
		}
	}
	return nil, fmt.Errorf("no matching remote found")
}

// FindByRepo returns the first Remote that points to a specific GitHub repository
func (r Remotes) FindByRepo(owner, name string) (*Remote, error) {
	for _, rem := range r {
		if strings.EqualFold(rem.RepoOwner(), owner) && strings.EqualFold(rem.RepoName(), name) {
			return rem, nil
		}
	}
	return nil, fmt.Errorf("no matching remote found")
}

// Filter remotes by given hostnames, maintains original order
func (r Remotes) FilterByHosts(hosts []string) Remotes {
	filtered := make(Remotes, 0)
	for _, rr := range r {
		for _, host := range hosts {
			if strings.EqualFold(rr.RepoHost(), host) {
				filtered = append(filtered, rr)
				break
			}
		}
	}
	return filtered
}

func (r Remotes) ResolvedRemote() (*Remote, error) {
	for _, rr := range r {
		if rr.Resolved != "" {
			return rr, nil
		}
	}
	return nil, fmt.Errorf("no resolved remote found")
}

func remoteNameSortScore(name string) int {
	switch strings.ToLower(name) {
	case "upstream":
		return 3
	case "github":
		return 2
	case "origin":
		return 1
	default:
		return 0
	}
}

// https://golang.org/pkg/sort/#Interface
func (r Remotes) Len() int      { return len(r) }
func (r Remotes) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r Remotes) Less(i, j int) bool {
	return remoteNameSortScore(r[i].Name) > remoteNameSortScore(r[j].Name)
}

// Remote represents a git remote mapped to a GitHub repository
type Remote struct {
	*git.Remote
	Repo ghrepo.Interface
}

// RepoName is the name of the GitHub repository
func (r Remote) RepoName() string {
	return r.Repo.RepoName()
}

// RepoOwner is the name of the GitHub account that owns the repo
func (r Remote) RepoOwner() string {
	return r.Repo.RepoOwner()
}

// RepoHost is the GitHub hostname that the remote points to
func (r Remote) RepoHost() string {
	return r.Repo.RepoHost()
}

type Translator interface {
	Translate(*url.URL) *url.URL
}

func TranslateRemotes(gitRemotes git.RemoteSet, translator Translator) (remotes Remotes) {
	for _, r := range gitRemotes {
		var repo ghrepo.Interface
		if r.FetchURL != nil {
			repo, _ = ghrepo.FromURL(translator.Translate(r.FetchURL))
		}
		if r.PushURL != nil && repo == nil {
			repo, _ = ghrepo.FromURL(translator.Translate(r.PushURL))
		}
		if repo == nil {
			continue
		}
		remotes = append(remotes, &Remote{
			Remote: r,
			Repo:   repo,
		})
	}
	return
}
