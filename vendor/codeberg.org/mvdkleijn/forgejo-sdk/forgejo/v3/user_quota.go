// Copyright 2026 The Forgejo Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	gen "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3/internal/generated/models"
	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3/models"
)

// QuotaSubject represents a quota limit subject for use with CheckMyQuota.
type QuotaSubject string

const (
	QuotaSubjectNone                          QuotaSubject = "none"
	QuotaSubjectSizeAll                       QuotaSubject = "size:all"
	QuotaSubjectSizeReposAll                  QuotaSubject = "size:repos:all"
	QuotaSubjectSizeReposPublic               QuotaSubject = "size:repos:public"
	QuotaSubjectSizeReposPrivate              QuotaSubject = "size:repos:private"
	QuotaSubjectSizeGitAll                    QuotaSubject = "size:git:all"
	QuotaSubjectSizeGitLFS                    QuotaSubject = "size:git:lfs"
	QuotaSubjectSizeAssetsAll                 QuotaSubject = "size:assets:all"
	QuotaSubjectSizeAssetsAttachmentsAll      QuotaSubject = "size:assets:attachments:all"
	QuotaSubjectSizeAssetsAttachmentsIssues   QuotaSubject = "size:assets:attachments:issues"
	QuotaSubjectSizeAssetsAttachmentsReleases QuotaSubject = "size:assets:attachments:releases"
	QuotaSubjectSizeAssetsArtifacts           QuotaSubject = "size:assets:artifacts"
	QuotaSubjectSizeAssetsPackagesAll         QuotaSubject = "size:assets:packages:all"
	QuotaSubjectSizeWiki                      QuotaSubject = "size:assets:wiki"
)

// GetMyQuota returns quota information for the authenticated user
func (c *Client) GetMyQuota(ctx context.Context) (models.QuotaInfo, Response, error) {
	var genQuota gen.QuotaInfo

	resp, err := c.getParsedResponseWithContext(ctx, "GET", "/user/quota", jsonHeader, nil, &genQuota)
	if err != nil {
		return models.QuotaInfo{}, resp, err
	}

	return mapQuotaInfo(&genQuota), resp, nil
}

// ListMyQuotaArtifactsOptions holds optional parameters for listing quota artifacts
type ListMyQuotaArtifactsOptions struct {
	ListOptions
}

// ListMyQuotaArtifacts lists artifacts counting towards the authenticated user's quota
func (c *Client) ListMyQuotaArtifacts(ctx context.Context, opt ListMyQuotaArtifactsOptions) ([]models.QuotaUsedArtifact, Response, error) {
	link := opt.getURLQuery().Encode()
	var genArtifacts []*gen.QuotaUsedArtifact

	path := "/user/quota/artifacts"
	if link != "" {
		path += "?" + link
	}

	resp, err := c.getParsedResponseWithContext(ctx, "GET", path, jsonHeader, nil, &genArtifacts)
	if err != nil {
		return nil, resp, err
	}

	artifacts := make([]models.QuotaUsedArtifact, 0, len(genArtifacts))
	for _, ga := range genArtifacts {
		if ga != nil {
			artifacts = append(artifacts, mapQuotaUsedArtifact(ga))
		}
	}

	return artifacts, resp, nil
}

// ListMyQuotaAttachmentsOptions holds optional parameters for listing quota attachments
type ListMyQuotaAttachmentsOptions struct {
	ListOptions
}

// ListMyQuotaAttachments lists attachments counting towards the authenticated user's quota
func (c *Client) ListMyQuotaAttachments(ctx context.Context, opt ListMyQuotaAttachmentsOptions) ([]models.QuotaUsedAttachment, Response, error) {
	link := opt.getURLQuery().Encode()
	var genAttachments []*gen.QuotaUsedAttachment

	path := "/user/quota/attachments"
	if link != "" {
		path += "?" + link
	}

	resp, err := c.getParsedResponseWithContext(ctx, "GET", path, jsonHeader, nil, &genAttachments)
	if err != nil {
		return nil, resp, err
	}

	attachments := make([]models.QuotaUsedAttachment, 0, len(genAttachments))
	for _, ga := range genAttachments {
		if ga != nil {
			attachments = append(attachments, mapQuotaUsedAttachment(ga))
		}
	}

	return attachments, resp, nil
}

// CheckMyQuota checks if the authenticated user is over quota.
func (c *Client) CheckMyQuota(ctx context.Context, subject QuotaSubject) (bool, Response, error) {
	s := strings.TrimSpace(string(subject))
	if s == "" {
		return false, Response{}, fmt.Errorf("quota subject is required")
	}

	data, resp, err := c.getResponseWithContext(ctx, "GET", "/user/quota/check?subject="+url.QueryEscape(s), nil, nil)
	if err != nil {
		return false, resp, err
	}

	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		return false, resp, fmt.Errorf("unexpected /user/quota/check response: %s", string(data))
	}

	return b, resp, nil
}

// ListMyQuotaPackagesOptions holds optional parameters for listing quota packages
type ListMyQuotaPackagesOptions struct {
	ListOptions
}

// ListMyQuotaPackages lists packages counting towards the authenticated user's quota
func (c *Client) ListMyQuotaPackages(ctx context.Context, opt ListMyQuotaPackagesOptions) ([]models.QuotaUsedPackage, Response, error) {
	link := opt.getURLQuery().Encode()
	var genPackages []*gen.QuotaUsedPackage

	path := "/user/quota/packages"
	if link != "" {
		path += "?" + link
	}

	resp, err := c.getParsedResponseWithContext(ctx, "GET", path, jsonHeader, nil, &genPackages)
	if err != nil {
		return nil, resp, err
	}

	packages := make([]models.QuotaUsedPackage, 0, len(genPackages))
	for _, gp := range genPackages {
		if gp != nil {
			packages = append(packages, mapQuotaUsedPackage(gp))
		}
	}

	return packages, resp, nil
}

// Mapping functions from generated models to public models

func mapQuotaInfo(in *gen.QuotaInfo) models.QuotaInfo {
	if in == nil {
		return models.QuotaInfo{}
	}

	info := models.QuotaInfo{
		Used: mapQuotaUsed(in.Used),
	}

	if len(in.Groups) > 0 {
		info.Groups = make([]models.QuotaGroup, 0, len(in.Groups))
		for _, gg := range in.Groups {
			if gg != nil {
				info.Groups = append(info.Groups, mapQuotaGroup(gg))
			}
		}
	}

	return info
}

func mapQuotaUsedArtifact(in *gen.QuotaUsedArtifact) models.QuotaUsedArtifact {
	return models.QuotaUsedArtifact{
		HTMLURL: in.HTMLURL,
		Name:    in.Name,
		Size:    in.Size,
	}
}

func mapQuotaUsedAttachment(in *gen.QuotaUsedAttachment) models.QuotaUsedAttachment {
	attachment := models.QuotaUsedAttachment{
		APIURL: in.APIURL,
		Name:   in.Name,
		Size:   in.Size,
	}

	if in.ContainedIn != nil {
		attachment.ContainedIn = &models.ContainedIn{
			APIURL:  in.ContainedIn.APIURL,
			HTMLURL: in.ContainedIn.HTMLURL,
		}
	}

	return attachment
}

func mapQuotaUsedPackage(in *gen.QuotaUsedPackage) models.QuotaUsedPackage {
	return models.QuotaUsedPackage{
		HTMLURL: in.HTMLURL,
		Name:    in.Name,
		Size:    in.Size,
		Type:    in.Type,
		Version: in.Version,
	}
}

func mapQuotaUsed(in *gen.QuotaUsed) models.QuotaUsed {
	if in == nil {
		return models.QuotaUsed{}
	}

	return models.QuotaUsed{
		Size: mapQuotaUsedSize(in.Size),
	}
}

func mapQuotaUsedSize(in *gen.QuotaUsedSize) models.QuotaUsedSize {
	if in == nil {
		return models.QuotaUsedSize{}
	}

	return models.QuotaUsedSize{
		Assets: mapQuotaUsedSizeAssets(in.Assets),
		Git:    mapQuotaUsedSizeGit(in.Git),
		Repos:  mapQuotaUsedSizeRepos(in.Repos),
	}
}

func mapQuotaUsedSizeAssets(in *gen.QuotaUsedSizeAssets) models.QuotaUsedSizeAssets {
	if in == nil {
		return models.QuotaUsedSizeAssets{}
	}

	return models.QuotaUsedSizeAssets{
		Artifacts:   in.Artifacts,
		Attachments: mapQuotaUsedSizeAssetsAttachments(in.Attachments),
		Packages:    mapQuotaUsedSizeAssetsPackages(in.Packages),
	}
}

func mapQuotaUsedSizeGit(in *gen.QuotaUsedSizeGit) models.QuotaUsedSizeGit {
	if in == nil {
		return models.QuotaUsedSizeGit{}
	}

	return models.QuotaUsedSizeGit{
		LFS: in.LFS,
	}
}

func mapQuotaUsedSizeRepos(in *gen.QuotaUsedSizeRepos) models.QuotaUsedSizeRepos {
	if in == nil {
		return models.QuotaUsedSizeRepos{}
	}

	return models.QuotaUsedSizeRepos{
		Private: in.Private,
		Public:  in.Public,
	}
}

func mapQuotaUsedSizeAssetsAttachments(in *gen.QuotaUsedSizeAssetsAttachments) models.QuotaUsedSizeAssetsAttachments {
	if in == nil {
		return models.QuotaUsedSizeAssetsAttachments{}
	}

	return models.QuotaUsedSizeAssetsAttachments{
		Issues:   in.Issues,
		Releases: in.Releases,
	}
}

func mapQuotaUsedSizeAssetsPackages(in *gen.QuotaUsedSizeAssetsPackages) models.QuotaUsedSizeAssetsPackages {
	if in == nil {
		return models.QuotaUsedSizeAssetsPackages{}
	}

	return models.QuotaUsedSizeAssetsPackages{
		All: in.All,
	}
}

func mapQuotaGroup(in *gen.QuotaGroup) models.QuotaGroup {
	group := models.QuotaGroup{
		Name: in.Name,
	}

	if len(in.Rules) > 0 {
		group.Rules = make([]models.QuotaRuleInfo, 0, len(in.Rules))
		for _, gr := range in.Rules {
			if gr != nil {
				group.Rules = append(group.Rules, mapQuotaRuleInfo(gr))
			}
		}
	}

	return group
}

func mapQuotaRuleInfo(in *gen.QuotaRuleInfo) models.QuotaRuleInfo {
	rule := models.QuotaRuleInfo{
		Name:  in.Name,
		Limit: in.Limit,
	}

	if len(in.Subjects) > 0 {
		rule.Subjects = make([]string, len(in.Subjects))
		copy(rule.Subjects, in.Subjects)
	}

	return rule
}
