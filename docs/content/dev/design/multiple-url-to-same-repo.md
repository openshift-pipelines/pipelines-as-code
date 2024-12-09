---
title: Matching multiple URLs to the same Repository CR
weight: 1
---

# Matching multiple URLs to the same Repository CR

## Background

The current implementation of the Repository CR allows specifying a single URL
for the repository. This is limiting in cases where the repository is mirrored
to multiple URLs. This document proposes a way to allow specifying multiple URLs
for the same repository.

## Goal

The goal is to allows specifying multiple URLs for the same repository in the
same namespace. All settings for all repositories are inside a single Repository
CR.

## Non Goals

- Allowing multiple URLs for the same repository in different namespaces.

## Proposal

- Extend the Repository CR to allow specifying a regexp for the URL.
- Extend the Repository CR to allow specifying a non-matching regexp.
- Extend the Pipelines-as-Code webhook to ensure that no other Repo CR
  configuration match the newly or updated Repository CR.

```yaml
spec:
  url: https://github.com/foo/.*
  non-url: https://github.com/foo/hello-*
```

This will match all repositories in the `foo` organization except the one
starting with `hello-*`

- Repository status will have a new field `url` that will show the
  matched URL.

## Discussion

The full security model of Pipelines-as-Code is based on the assumption that the
first one wins. Since we don't have a way (or we decided not to for the
sake of making it simple) to know who is the real owner of a GitHub (or
others) repository, we assume that the first one that creates the Repository
is the owner of the repository. It is up to the cluster administrator to make
sure which team that has access to a namespace have the proper Repository CR
matching the GitHub Repository.

If we start letting people making a regexp for the URL, we need to make sure
they are not able to hijack other repositories.

Example scenario:

- User A creates a Repository CR with a regexp for the URL that matches
  all repositories in the organization.
- User B creates another Repository CR with a regexp for the URL that as well
  matches all repositories in the organization with a different syntax (regexp
  have many ways to express the same thing).

The webhook will need to make sure that when User B create that Repository CR to
deny it since User A had already have a match for it. It is up to the
Users/Teams to communicate to have the regexps works (using `non-url` setting
for example) for some repositories and not the others.

Second scenario:

- User A has created a Repository CR matching URL: <https://github.com/org/repos>
- User B has a Repository CR matching URL: <https://github.com/org/otherrepos>
- User A decide to update their Repo CR with: <https://github.com/org/.*>

The webhook will deny User A since we have a repository CR in user B already
matching the new update.

## CLI

- [ ] The CLI will probably not need to handle such advanced use case, this is up to
  the user to directly edit the `Repository` CR Yaml.

## UI

- OpenShift Console will need to be updated I believe to be able to show a status for multiple URL match (TBC)

## Tekton Results

- TBD

## Alternatives

We could extend the global `ConfigMap` setting `auto-configure-new-github-repo` to
only allows a certain regexps and have a `Repository` template.

The problem with this is that:

- It is specific to GitHub
- The settings is admin level.
- The Repository CR are not allowed to be customized by the user.
- The regexps may need to be adjusted and it will only works for a specific
  scenario.

The advantages is that UI and tools would not need to be adjusted.
