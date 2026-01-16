// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT.

// Package forgejostructs contains Gitea webhook payload types.
// These types are copied from code.gitea.io/gitea/modules/structs v1.25.3.
// to avoid importing the entire Gitea codebase and its massive dependency tree.
package forgejostructs

import (
	"encoding/json"
	"time"
)

// -----------------------------------------------------------------------------
// User types.
// -----------------------------------------------------------------------------

// User represents a user.
type User struct {
	ID        int64  `json:"id"`
	UserName  string `json:"login"`
	LoginName string `json:"login_name"`
	SourceID  int64  `json:"source_id"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Language  string `json:"language"`
	IsAdmin   bool   `json:"is_admin"`
	// swagger:strfmt date-time.
	LastLogin time.Time `json:"last_login"`
	// swagger:strfmt date-time.
	Created       time.Time `json:"created"`
	Restricted    bool      `json:"restricted"`
	IsActive      bool      `json:"active"`
	ProhibitLogin bool      `json:"prohibit_login"`
	Location      string    `json:"location"`
	Website       string    `json:"website"`
	Description   string    `json:"description"`
	Visibility    string    `json:"visibility"`
	Followers     int       `json:"followers_count"`
	Following     int       `json:"following_count"`
	StarredRepos  int       `json:"starred_repos_count"`
}

// MarshalJSON implements the json.Marshaler interface for User, adding field(s) for backward compatibility.
func (u User) MarshalJSON() ([]byte, error) {
	type shadow User
	return json.Marshal(struct {
		shadow
		CompatUserName string `json:"username"`
	}{shadow(u), u.UserName})
}

// userAlias is used to avoid recursion in UnmarshalJSON.
type userAlias User

// userWithCompat includes the backwards-compatible username field that Gitea sends in webhook payloads.
type userWithCompat struct {
	userAlias
	CompatUserName string `json:"username"`
}

// UnmarshalJSON implements json.Unmarshaler, reading the "username" field into UserName if "login" is empty.
func (u *User) UnmarshalJSON(data []byte) error {
	var compat userWithCompat
	if err := json.Unmarshal(data, &compat); err != nil {
		return err
	}
	*u = User(compat.userAlias)
	if u.UserName == "" && compat.CompatUserName != "" {
		u.UserName = compat.CompatUserName
	}
	return nil
}

// -----------------------------------------------------------------------------
// Permission types.
// -----------------------------------------------------------------------------

// Permission represents a set of permissions.
type Permission struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// -----------------------------------------------------------------------------
// Repository types.
// -----------------------------------------------------------------------------

// InternalTracker represents settings for internal tracker.
type InternalTracker struct {
	EnableTimeTracker                bool `json:"enable_time_tracker"`
	AllowOnlyContributorsToTrackTime bool `json:"allow_only_contributors_to_track_time"`
	EnableIssueDependencies          bool `json:"enable_issue_dependencies"`
}

// ExternalTracker represents settings for external tracker.
type ExternalTracker struct {
	ExternalTrackerURL           string `json:"external_tracker_url"`
	ExternalTrackerFormat        string `json:"external_tracker_format"`
	ExternalTrackerStyle         string `json:"external_tracker_style"`
	ExternalTrackerRegexpPattern string `json:"external_tracker_regexp_pattern"`
}

// ExternalWiki represents setting for external wiki.
type ExternalWiki struct {
	ExternalWikiURL string `json:"external_wiki_url"`
}

// RepoTransfer represents a pending repo transfer.
type RepoTransfer struct {
	Doer      *User   `json:"doer"`
	Recipient *User   `json:"recipient"`
	Teams     []*Team `json:"teams"`
}

// Repository represents a repository.
type Repository struct {
	ID                            int64            `json:"id"`
	Owner                         *User            `json:"owner"`
	Name                          string           `json:"name"`
	FullName                      string           `json:"full_name"`
	Description                   string           `json:"description"`
	Empty                         bool             `json:"empty"`
	Private                       bool             `json:"private"`
	Fork                          bool             `json:"fork"`
	Template                      bool             `json:"template"`
	Parent                        *Repository      `json:"parent,omitempty"`
	Mirror                        bool             `json:"mirror"`
	Size                          int              `json:"size"`
	Language                      string           `json:"language"`
	LanguagesURL                  string           `json:"languages_url"`
	HTMLURL                       string           `json:"html_url"`
	URL                           string           `json:"url"`
	Link                          string           `json:"link"`
	SSHURL                        string           `json:"ssh_url"`
	CloneURL                      string           `json:"clone_url"`
	OriginalURL                   string           `json:"original_url"`
	Website                       string           `json:"website"`
	Stars                         int              `json:"stars_count"`
	Forks                         int              `json:"forks_count"`
	Watchers                      int              `json:"watchers_count"`
	OpenIssues                    int              `json:"open_issues_count"`
	OpenPulls                     int              `json:"open_pr_counter"`
	Releases                      int              `json:"release_counter"`
	DefaultBranch                 string           `json:"default_branch"`
	Archived                      bool             `json:"archived"`
	Created                       time.Time        `json:"created_at"`
	Updated                       time.Time        `json:"updated_at"`
	ArchivedAt                    time.Time        `json:"archived_at"`
	Permissions                   *Permission      `json:"permissions,omitempty"`
	HasCode                       bool             `json:"has_code"`
	HasIssues                     bool             `json:"has_issues"`
	InternalTracker               *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker               *ExternalTracker `json:"external_tracker,omitempty"`
	HasWiki                       bool             `json:"has_wiki"`
	ExternalWiki                  *ExternalWiki    `json:"external_wiki,omitempty"`
	HasPullRequests               bool             `json:"has_pull_requests"`
	HasProjects                   bool             `json:"has_projects"`
	ProjectsMode                  string           `json:"projects_mode"`
	HasReleases                   bool             `json:"has_releases"`
	HasPackages                   bool             `json:"has_packages"`
	HasActions                    bool             `json:"has_actions"`
	IgnoreWhitespaceConflicts     bool             `json:"ignore_whitespace_conflicts"`
	AllowMerge                    bool             `json:"allow_merge_commits"`
	AllowRebase                   bool             `json:"allow_rebase"`
	AllowRebaseMerge              bool             `json:"allow_rebase_explicit"`
	AllowSquash                   bool             `json:"allow_squash_merge"`
	AllowFastForwardOnly          bool             `json:"allow_fast_forward_only_merge"`
	AllowRebaseUpdate             bool             `json:"allow_rebase_update"`
	AllowManualMerge              bool             `json:"allow_manual_merge"`
	AutodetectManualMerge         bool             `json:"autodetect_manual_merge"`
	DefaultDeleteBranchAfterMerge bool             `json:"default_delete_branch_after_merge"`
	DefaultMergeStyle             string           `json:"default_merge_style"`
	DefaultAllowMaintainerEdit    bool             `json:"default_allow_maintainer_edit"`
	AvatarURL                     string           `json:"avatar_url"`
	Internal                      bool             `json:"internal"`
	MirrorInterval                string           `json:"mirror_interval"`
	ObjectFormatName              string           `json:"object_format_name"`
	MirrorUpdated                 time.Time        `json:"mirror_updated"`
	RepoTransfer                  *RepoTransfer    `json:"repo_transfer,omitempty"`
	Topics                        []string         `json:"topics"`
	Licenses                      []string         `json:"licenses"`
}

// -----------------------------------------------------------------------------
// Label types.
// -----------------------------------------------------------------------------

// Label a label to an issue or a pr.
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Exclusive   bool   `json:"exclusive"`
	IsArchived  bool   `json:"is_archived"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// -----------------------------------------------------------------------------
// Milestone types.
// -----------------------------------------------------------------------------

// StateType issue state type.
type StateType string

const (
	StateOpen   StateType = "open"
	StateClosed StateType = "closed"
	StateAll    StateType = "all"
)

// Milestone milestone is a collection of issues on one repository.
type Milestone struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	State        StateType  `json:"state"`
	OpenIssues   int        `json:"open_issues"`
	ClosedIssues int        `json:"closed_issues"`
	Created      time.Time  `json:"created_at"`
	Updated      *time.Time `json:"updated_at"`
	Closed       *time.Time `json:"closed_at"`
	Deadline     *time.Time `json:"due_on"`
}

// -----------------------------------------------------------------------------
// Comment types.
// -----------------------------------------------------------------------------

// Comment represents a comment on a commit or issue.
type Comment struct {
	ID               int64         `json:"id"`
	HTMLURL          string        `json:"html_url"`
	PRURL            string        `json:"pull_request_url"`
	IssueURL         string        `json:"issue_url"`
	Poster           *User         `json:"user"`
	OriginalAuthor   string        `json:"original_author"`
	OriginalAuthorID int64         `json:"original_author_id"`
	Body             string        `json:"body"`
	Attachments      []*Attachment `json:"assets"`
	Created          time.Time     `json:"created_at"`
	Updated          time.Time     `json:"updated_at"`
}

// -----------------------------------------------------------------------------
// Attachment types.
// -----------------------------------------------------------------------------

// Attachment a generic attachment.
type Attachment struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	DownloadCount int64     `json:"download_count"`
	Created       time.Time `json:"created_at"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"browser_download_url"`
}

// -----------------------------------------------------------------------------
// Issue types.
// -----------------------------------------------------------------------------

// PullRequestMeta PR info if an issue is a PR.
type PullRequestMeta struct {
	HasMerged        bool       `json:"merged"`
	Merged           *time.Time `json:"merged_at"`
	IsWorkInProgress bool       `json:"draft"`
	HTMLURL          string     `json:"html_url"`
}

// RepositoryMeta basic repository information.
type RepositoryMeta struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

// Issue represents an issue in a repository.
type Issue struct {
	ID               int64            `json:"id"`
	URL              string           `json:"url"`
	HTMLURL          string           `json:"html_url"`
	Index            int64            `json:"number"`
	Poster           *User            `json:"user"`
	OriginalAuthor   string           `json:"original_author"`
	OriginalAuthorID int64            `json:"original_author_id"`
	Title            string           `json:"title"`
	Body             string           `json:"body"`
	Ref              string           `json:"ref"`
	Attachments      []*Attachment    `json:"assets"`
	Labels           []*Label         `json:"labels"`
	Milestone        *Milestone       `json:"milestone"`
	Assignee         *User            `json:"assignee"`
	Assignees        []*User          `json:"assignees"`
	State            StateType        `json:"state"`
	IsLocked         bool             `json:"is_locked"`
	Comments         int              `json:"comments"`
	Created          time.Time        `json:"created_at"`
	Updated          time.Time        `json:"updated_at"`
	Closed           *time.Time       `json:"closed_at"`
	Deadline         *time.Time       `json:"due_date"`
	TimeEstimate     int64            `json:"time_estimate"`
	PullRequest      *PullRequestMeta `json:"pull_request"`
	Repo             *RepositoryMeta  `json:"repository"`
	PinOrder         int              `json:"pin_order"`
}

// -----------------------------------------------------------------------------
// Pull Request types.
// -----------------------------------------------------------------------------

// PRBranchInfo information about a branch.
type PRBranchInfo struct {
	Name       string      `json:"label"`
	Ref        string      `json:"ref"`
	Sha        string      `json:"sha"`
	RepoID     int64       `json:"repo_id"`
	Repository *Repository `json:"repo"`
}

// PullRequest represents a pull request.
type PullRequest struct {
	ID                      int64         `json:"id"`
	URL                     string        `json:"url"`
	Index                   int64         `json:"number"`
	Poster                  *User         `json:"user"`
	Title                   string        `json:"title"`
	Body                    string        `json:"body"`
	Labels                  []*Label      `json:"labels"`
	Milestone               *Milestone    `json:"milestone"`
	Assignee                *User         `json:"assignee"`
	Assignees               []*User       `json:"assignees"`
	RequestedReviewers      []*User       `json:"requested_reviewers"`
	RequestedReviewersTeams []*Team       `json:"requested_reviewers_teams"`
	State                   StateType     `json:"state"`
	Draft                   bool          `json:"draft"`
	IsLocked                bool          `json:"is_locked"`
	Comments                int           `json:"comments"`
	ReviewComments          int           `json:"review_comments,omitempty"`
	Additions               *int          `json:"additions,omitempty"`
	Deletions               *int          `json:"deletions,omitempty"`
	ChangedFiles            *int          `json:"changed_files,omitempty"`
	HTMLURL                 string        `json:"html_url"`
	DiffURL                 string        `json:"diff_url"`
	PatchURL                string        `json:"patch_url"`
	Mergeable               bool          `json:"mergeable"`
	HasMerged               bool          `json:"merged"`
	Merged                  *time.Time    `json:"merged_at"`
	MergedCommitID          *string       `json:"merge_commit_sha"`
	MergedBy                *User         `json:"merged_by"`
	AllowMaintainerEdit     bool          `json:"allow_maintainer_edit"`
	Base                    *PRBranchInfo `json:"base"`
	Head                    *PRBranchInfo `json:"head"`
	MergeBase               string        `json:"merge_base"`
	Deadline                *time.Time    `json:"due_date"`
	Created                 *time.Time    `json:"created_at"`
	Updated                 *time.Time    `json:"updated_at"`
	Closed                  *time.Time    `json:"closed_at"`
	PinOrder                int           `json:"pin_order"`
}

// -----------------------------------------------------------------------------
// Release types.
// -----------------------------------------------------------------------------

// Release represents a repository release.
type Release struct {
	ID           int64         `json:"id"`
	TagName      string        `json:"tag_name"`
	Target       string        `json:"target_commitish"`
	Title        string        `json:"name"`
	Note         string        `json:"body"`
	URL          string        `json:"url"`
	HTMLURL      string        `json:"html_url"`
	TarURL       string        `json:"tarball_url"`
	ZipURL       string        `json:"zipball_url"`
	UploadURL    string        `json:"upload_url"`
	IsDraft      bool          `json:"draft"`
	IsPrerelease bool          `json:"prerelease"`
	CreatedAt    time.Time     `json:"created_at"`
	PublishedAt  time.Time     `json:"published_at"`
	Publisher    *User         `json:"author"`
	Attachments  []*Attachment `json:"assets"`
}

// -----------------------------------------------------------------------------
// Organization types.
// -----------------------------------------------------------------------------

// Organization represents an organization.
type Organization struct {
	ID                        int64  `json:"id"`
	Name                      string `json:"name"`
	FullName                  string `json:"full_name"`
	Email                     string `json:"email"`
	AvatarURL                 string `json:"avatar_url"`
	Description               string `json:"description"`
	Website                   string `json:"website"`
	Location                  string `json:"location"`
	Visibility                string `json:"visibility"`
	RepoAdminChangeTeamAccess bool   `json:"repo_admin_change_team_access"`
	UserName                  string `json:"username"`
}

// -----------------------------------------------------------------------------
// Team types.
// -----------------------------------------------------------------------------

// Team represents a team in an organization.
type Team struct {
	ID                      int64             `json:"id"`
	Name                    string            `json:"name"`
	Description             string            `json:"description"`
	Organization            *Organization     `json:"organization"`
	IncludesAllRepositories bool              `json:"includes_all_repositories"`
	Permission              string            `json:"permission"`
	Units                   []string          `json:"units"`
	UnitsMap                map[string]string `json:"units_map"`
	CanCreateOrgRepo        bool              `json:"can_create_org_repo"`
}

// -----------------------------------------------------------------------------
// Hook/Webhook types.
// -----------------------------------------------------------------------------

// PayloadUser represents the author or committer of a commit.
type PayloadUser struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserName string `json:"username"`
}

// PayloadCommitVerification represents the GPG verification of a commit.
type PayloadCommitVerification struct {
	Verified  bool         `json:"verified"`
	Reason    string       `json:"reason"`
	Signature string       `json:"signature"`
	Signer    *PayloadUser `json:"signer"`
	Payload   string       `json:"payload"`
}

// PayloadCommit represents a commit.
type PayloadCommit struct {
	ID           string                     `json:"id"`
	Message      string                     `json:"message"`
	URL          string                     `json:"url"`
	Author       *PayloadUser               `json:"author"`
	Committer    *PayloadUser               `json:"committer"`
	Verification *PayloadCommitVerification `json:"verification"`
	Timestamp    time.Time                  `json:"timestamp"`
	Added        []string                   `json:"added"`
	Removed      []string                   `json:"removed"`
	Modified     []string                   `json:"modified"`
}

// PusherType define the type to push.
type PusherType string

const (
	PusherTypeUser PusherType = "user"
)

// HookIssueAction defines hook issue action.
type HookIssueAction string

const (
	HookIssueOpened               HookIssueAction = "opened"
	HookIssueClosed               HookIssueAction = "closed"
	HookIssueReOpened             HookIssueAction = "reopened"
	HookIssueEdited               HookIssueAction = "edited"
	HookIssueDeleted              HookIssueAction = "deleted"
	HookIssueAssigned             HookIssueAction = "assigned"
	HookIssueUnassigned           HookIssueAction = "unassigned"
	HookIssueLabelUpdated         HookIssueAction = "label_updated"
	HookIssueLabelCleared         HookIssueAction = "label_cleared"
	HookIssueSynchronized         HookIssueAction = "synchronized"
	HookIssueMilestoned           HookIssueAction = "milestoned"
	HookIssueDemilestoned         HookIssueAction = "demilestoned"
	HookIssueReviewed             HookIssueAction = "reviewed"
	HookIssueReviewRequested      HookIssueAction = "review_requested"
	HookIssueReviewRequestRemoved HookIssueAction = "review_request_removed"
)

// HookIssueCommentAction defines hook issue comment action.
type HookIssueCommentAction string

const (
	HookIssueCommentCreated HookIssueCommentAction = "created"
	HookIssueCommentEdited  HookIssueCommentAction = "edited"
	HookIssueCommentDeleted HookIssueCommentAction = "deleted"
)

// HookReleaseAction defines hook release action type.
type HookReleaseAction string

const (
	HookReleasePublished HookReleaseAction = "published"
	HookReleaseUpdated   HookReleaseAction = "updated"
	HookReleaseDeleted   HookReleaseAction = "deleted"
)

// HookRepoAction defines hook repository action type.
type HookRepoAction string

const (
	HookRepoCreated HookRepoAction = "created"
	HookRepoDeleted HookRepoAction = "deleted"
)

// ChangesFromPayload represents the previous value before a change.
type ChangesFromPayload struct {
	From string `json:"from"`
}

// ChangesPayload represents the payload information of issue change.
type ChangesPayload struct {
	Title         *ChangesFromPayload `json:"title,omitempty"`
	Body          *ChangesFromPayload `json:"body,omitempty"`
	Ref           *ChangesFromPayload `json:"ref,omitempty"`
	AddedLabels   []*Label            `json:"added_labels"`
	RemovedLabels []*Label            `json:"removed_labels"`
}

// ReviewPayload represents review information.
type ReviewPayload struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// -----------------------------------------------------------------------------
// Webhook Payload types.
// -----------------------------------------------------------------------------

// CreatePayload represents a payload information of create event.
type CreatePayload struct {
	Sha     string      `json:"sha"`
	Ref     string      `json:"ref"`
	RefType string      `json:"ref_type"`
	Repo    *Repository `json:"repository"`
	Sender  *User       `json:"sender"`
}

// DeletePayload represents delete payload.
type DeletePayload struct {
	Ref        string      `json:"ref"`
	RefType    string      `json:"ref_type"`
	PusherType PusherType  `json:"pusher_type"`
	Repo       *Repository `json:"repository"`
	Sender     *User       `json:"sender"`
}

// ForkPayload represents fork payload.
type ForkPayload struct {
	Forkee *Repository `json:"forkee"`
	Repo   *Repository `json:"repository"`
	Sender *User       `json:"sender"`
}

// PushPayload represents a payload information of push event.
type PushPayload struct {
	Ref          string           `json:"ref"`
	Before       string           `json:"before"`
	After        string           `json:"after"`
	CompareURL   string           `json:"compare_url"`
	Commits      []*PayloadCommit `json:"commits"`
	TotalCommits int              `json:"total_commits"`
	HeadCommit   *PayloadCommit   `json:"head_commit"`
	Repo         *Repository      `json:"repository"`
	Pusher       *User            `json:"pusher"`
	Sender       *User            `json:"sender"`
}

// IssuePayload represents the payload information that is sent along with an issue event.
type IssuePayload struct {
	Action     HookIssueAction `json:"action"`
	Index      int64           `json:"number"`
	Changes    *ChangesPayload `json:"changes,omitempty"`
	Issue      *Issue          `json:"issue"`
	Repository *Repository     `json:"repository"`
	Sender     *User           `json:"sender"`
	CommitID   string          `json:"commit_id"`
}

// IssueCommentPayload represents a payload information of issue comment event.
type IssueCommentPayload struct {
	Action      HookIssueCommentAction `json:"action"`
	Issue       *Issue                 `json:"issue"`
	PullRequest *PullRequest           `json:"pull_request,omitempty"`
	Comment     *Comment               `json:"comment"`
	Changes     *ChangesPayload        `json:"changes,omitempty"`
	Repository  *Repository            `json:"repository"`
	Sender      *User                  `json:"sender"`
	IsPull      bool                   `json:"is_pull"`
}

// PullRequestPayload represents a payload information of pull request event.
type PullRequestPayload struct {
	Action            HookIssueAction `json:"action"`
	Index             int64           `json:"number"`
	Changes           *ChangesPayload `json:"changes,omitempty"`
	PullRequest       *PullRequest    `json:"pull_request"`
	RequestedReviewer *User           `json:"requested_reviewer"`
	Repository        *Repository     `json:"repository"`
	Sender            *User           `json:"sender"`
	CommitID          string          `json:"commit_id"`
	Review            *ReviewPayload  `json:"review"`
}

// ReleasePayload represents a payload information of release event.
type ReleasePayload struct {
	Action     HookReleaseAction `json:"action"`
	Release    *Release          `json:"release"`
	Repository *Repository       `json:"repository"`
	Sender     *User             `json:"sender"`
}

// RepositoryPayload payload for repository webhooks.
type RepositoryPayload struct {
	Action       HookRepoAction `json:"action"`
	Repository   *Repository    `json:"repository"`
	Organization *User          `json:"organization"`
	Sender       *User          `json:"sender"`
}
