---
title: Pipelines-as-Code Release Process
---

This page describes the steps to create a new Pipelines-as-Code release.

## Prerequisites

* Clear out any PRs that need to be merged.
* Wait for CI to complete.
* Verify the PAC CI cluster is up.
* Verify that you have GPG signing [set up](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification) for your commits.

## Tagging the Release

Choose between a major, minor, or patch release version.

For example, to release version 1.2.3, tag it locally:

```shell
git tag v1.2.3
```

Push it directly to the repository (you need write access):

```shell
% git push --no-verify git@github.com:openshift-pipelines/pipelines-as-code refs/tags/1.2.3
```

## Monitoring the Release

Once the tag is pushed, follow the release pipeline on the PAC cluster:

`tkn pr logs -n pipelines-as-code-ci -Lf`

After a while (gorelease takes some time), the new version should appear as a pre-release at:

<https://github.com/openshift-pipelines/pipelines-as-code/releases>

## Publishing the Release

Edit the release notes following the same format as previous releases, with a snippet highlighting the key changes.

If you use AI to draft release notes:

* Verify the content, as it may contain mistakes.
* Avoid overusing emojis. Keep the tone professional.
* Categorize changes properly. AI may sometimes expose internal changes as major features.

Announce the release on Slack (upstream/downstream) and Twitter.

## Troubleshooting

If you need to re-trigger the release process due to system or other issues:

```shell
   git tag --force v1.2.3
   git push --force git@github.com:openshift-pipelines/pipelines-as-code v1.2.3
```

Common issues:

* The GitHub token may be expired or badly generated with a trailing `\n`.
* If you did not run `git fetch -a origin` before tagging, you may not have the latest commits from `origin/main`.
