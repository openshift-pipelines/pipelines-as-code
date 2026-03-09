---
title: Policy on actions
weight: 5
---

This page explains how to restrict which users can trigger pipelines or approve pull requests for CI. Use policies when you need fine-grained control over who can start PipelineRuns beyond the default collaborator and owner checks.

## Overview

Pipelines-as-Code uses policies to control which actions specific team members can perform
within an organization. You configure these policies in the Repository CR, referencing team names defined on your Git provider (currently GitHub and Forgejo).

{{< support_matrix github_app="true" github_webhook="true" forgejo="true" gitlab="false" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

## Supported actions

* `pull_request` -- Controls who can trigger CI on their own pull requests. When you specify
  a team, only members of that team can trigger CI, regardless
  of whether they are repository or organization owners or
  collaborators. Members listed in the `OWNERS` file can still
  trigger CI.

* `ok_to_test` -- Controls who can authorize CI for external contributors by commenting `/ok-to-test` on a pull request.
  This also applies to `/test` and `/retest`
  commands. Note that `/retest` only re-triggers failed PipelineRuns. This action takes precedence over the `pull_request` action.

## Configuring policies in the Repository CR

To set up policies, add a `settings.policy` block to your Repository CR:

```yaml
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repository1
spec:
  url: "https://github.com/org/repo"
  settings:
    policy:
      ok_to_test:
        - ci-admins
      pull_request:
        - ci-users
```

In this example:

* Members of the `ci-admins` team can authorize other users to run the CI on
  pull requests.
* Members of the `ci-users` team can run CI on their own pull requests.
