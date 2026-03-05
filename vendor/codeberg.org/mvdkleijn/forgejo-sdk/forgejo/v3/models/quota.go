// Copyright 2026 The Forgejo Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// QuotaUsed represents the quota usage of a user
type QuotaUsed struct {
	Size QuotaUsedSize `json:"size"`
}

// QuotaUsedSize represents the size-based quota usage of a user
type QuotaUsedSize struct {
	Assets QuotaUsedSizeAssets `json:"assets"`
	Git    QuotaUsedSizeGit    `json:"git"`
	Repos  QuotaUsedSizeRepos  `json:"repos"`
}

// QuotaInfo represents information about a user's quota
type QuotaInfo struct {
	Groups []QuotaGroup `json:"groups,omitempty"`
	Used   QuotaUsed    `json:"used"`
}

// QuotaGroup represents a quota group
type QuotaGroup struct {
	Name  string          `json:"name,omitempty"`
	Rules []QuotaRuleInfo `json:"rules"`
}

// QuotaRuleInfo contains information about a quota rule
type QuotaRuleInfo struct {
	// Name of the rule (only shown to admins)
	Name     string   `json:"name,omitempty"`
	Limit    int64    `json:"limit,omitempty"`
	Subjects []string `json:"subjects,omitempty"`
}

// QuotaUsedSizeAssets represents the size-based asset usage of a user
type QuotaUsedSizeAssets struct {
	// Storage size used for the user's artifacts
	Artifacts   int64                          `json:"artifacts,omitempty"`
	Attachments QuotaUsedSizeAssetsAttachments `json:"attachments"`
	Packages    QuotaUsedSizeAssetsPackages    `json:"packages"`
}

// QuotaUsedSizeGit represents the size-based git (lfs) quota usage of a user
type QuotaUsedSizeGit struct {
	// Storage size of the user's Git LFS objects
	LFS int64 `json:"LFS,omitempty"`
}

// QuotaUsedSizeRepos represents the size-based repository quota usage of a user
type QuotaUsedSizeRepos struct {
	// Storage size of the user's private repositories
	Private int64 `json:"private,omitempty"`
	// Storage size of the user's public repositories
	Public int64 `json:"public,omitempty"`
}

// QuotaUsedSizeAssetsAttachments represents the size-based attachment quota usage of a user
type QuotaUsedSizeAssetsAttachments struct {
	// Storage size used for the user's issue & comment attachments
	Issues int64 `json:"issues,omitempty"`
	// Storage size used for the user's release attachments
	Releases int64 `json:"releases,omitempty"`
}

// QuotaUsedSizeAssetsPackages represents the size-based package quota usage of a user
type QuotaUsedSizeAssetsPackages struct {
	// Storage size used for the user's packages
	All int64 `json:"all,omitempty"`
}

// QuotaUsedArtifact represents an artifact counting towards a user's quota
type QuotaUsedArtifact struct {
	// HTML URL to the action run containing the artifact
	HTMLURL string `json:"html_url,omitempty"`
	// Name of the artifact
	Name string `json:"name,omitempty"`
	// Size of the artifact (compressed)
	Size int64 `json:"size,omitempty"`
}

// QuotaUsedAttachment represents an attachment counting towards a user's quota
// ContainedIn describes the issue or release this attachment belongs to.
// It is nil if the attachment is not contained in any issue or release.
type QuotaUsedAttachment struct {
	// API URL for the attachment
	APIURL string `json:"api_url,omitempty"`
	// Filename of the attachment
	Name string `json:"name,omitempty"`
	// Size of the attachment (in bytes)
	Size int64 `json:"size,omitempty"`
	// contained in
	ContainedIn *ContainedIn `json:"contained_in,omitempty"`
}

// ContainedIn Context for the attachment: URLs to the containing object
type ContainedIn struct {
	// API URL for the object that contains this attachment
	APIURL string `json:"api_url,omitempty"`
	// HTML URL for the object that contains this attachment
	HTMLURL string `json:"html_url,omitempty"`
}

// QuotaUsedPackage represents a package counting towards a user's quota
type QuotaUsedPackage struct {
	// HTML URL to the package version
	HTMLURL string `json:"html_url,omitempty"`
	// Name of the package
	Name string `json:"name,omitempty"`
	// Size of the package version
	Size int64 `json:"size,omitempty"`
	// Type of the package
	Type string `json:"type,omitempty"`
	// Version of the package
	Version string `json:"version,omitempty"`
}
