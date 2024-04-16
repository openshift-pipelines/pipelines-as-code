package azuredevops

import (
	"time"
)

type PullRequestEventResource struct {
	Repository            Repository `json:"repository"`
	PullRequestID         int        `json:"pullRequestId"`
	CodeReviewID          int        `json:"codeReviewId,omitempty"`
	Status                string     `json:"status"`
	CreatedBy             User       `json:"createdBy"`
	CreationDate          CustomTime `json:"creationDate"`
	Title                 string     `json:"title"`
	Description           string     `json:"description,omitempty"`
	SourceRefName         string     `json:"sourceRefName"`
	TargetRefName         string     `json:"targetRefName"`
	MergeStatus           string     `json:"mergeStatus"`
	IsDraft               bool       `json:"isDraft,omitempty"`
	MergeID               string     `json:"mergeId"`
	LastMergeSourceCommit Commit     `json:"lastMergeSourceCommit"`
	LastMergeTargetCommit Commit     `json:"lastMergeTargetCommit"`
	LastMergeCommit       Commit     `json:"lastMergeCommit,omitempty"`
	Reviewers             []User     `json:"reviewers"`
	URL                   string     `json:"url"`
	Links                 Links      `json:"_links"`
	SupportsIterations    bool       `json:"supportsIterations,omitempty"`
	ArtifactID            string     `json:"artifactId,omitempty"`
}

type PushEventResource struct {
	Commits    []Commit    `json:"commits"`
	RefUpdates []RefUpdate `json:"refUpdates"`
	Repository Repository  `json:"repository"`
	PushedBy   User        `json:"pushedBy"`
	PushID     int         `json:"pushId"`
	Date       CustomTime  `json:"date"`
	URL        string      `json:"url"`
}

type Commit struct {
	CommitID  string `json:"commitId,omitempty"`
	Author    User   `json:"author,omitempty"`
	Committer User   `json:"committer,omitempty"`
	Comment   string `json:"comment,omitempty"`
	URL       string `json:"url,omitempty"`
}

type RefUpdate struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

type Repository struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	URL             string  `json:"url"`
	Project         Project `json:"project"`
	DefaultBranch   string  `json:"defaultBranch,omitempty"`
	Size            *int    `json:"size,omitempty"`
	RemoteURL       string  `json:"remoteUrl"`
	SSHURL          *string `json:"sshUrl,omitempty"`
	WebURL          *string `json:"webUrl,omitempty"`
	IsDisabled      *bool   `json:"isDisabled,omitempty"`
	IsInMaintenance *bool   `json:"isInMaintenance,omitempty"`
}

type Project struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	State          string     `json:"state"`
	Revision       int        `json:"revision,omitempty"`
	Visibility     string     `json:"visibility"`
	LastUpdateTime CustomTime `json:"lastUpdateTime"`
}

type User struct {
	Name        string     `json:"name,omitempty"`
	Email       string     `json:"email,omitempty"`
	Date        CustomTime `json:"date,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	URL         string     `json:"url,omitempty"`
	Links       Links      `json:"_links,omitempty"`
	ID          string     `json:"id,omitempty"`
	UniqueName  string     `json:"uniqueName,omitempty"`
	ImageURL    string     `json:"imageUrl,omitempty"`
	Descriptor  string     `json:"descriptor,omitempty"`
}

type ResourceContainers struct {
	Collection Container `json:"collection"`
	Account    Container `json:"account"`
	Project    Container `json:"project"`
}

type Container struct {
	ID string `json:"id"`
}

type Links struct {
	Web      Href `json:"web,omitempty"`
	Statuses Href `json:"statuses,omitempty"`
	Avatar   Href `json:"avatar,omitempty"`
}

type Href struct {
	Href string `json:"href"`
}

type CustomTime struct {
	time.Time
}

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	s = s[1 : len(s)-1] // Remove quotes
	if s == "0001-01-01T00:00:00" || s == "" {
		ct.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}
