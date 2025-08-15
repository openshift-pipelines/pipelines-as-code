---
title: Policy on actions
weight: 50
---

# Policy on Pipelines-as-Code Actions

Pipelines-as-Code uses policies to control which actions can be performed by
users who belong to specific teams within an organization, as defined on GitHub
or other supported Git providers (currently GitHub and Gitea).

{{< support_matrix github_app="true" github_webhook="true" gitea="true" gitlab="false" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

## Supported Actions

* `pull_request` - This action triggers the CI in Pipelines-as-Code. Specifying
  a team restricts the ability to trigger CI to members of that team, regardless
  of whether they are repository or organization owners or
  collaborators. However, members listed in the `OWNERS` file are still
  permitted to trigger the CI.

* `ok_to_test` - This action allows users who are members of the specified team
  to trigger the CI for a pull request by commenting `/ok-to-test`. This enables
  CI to run on pull requests submitted by contributors who are not collaborators
  of the repository or organization. It also applies to `/test` and `/retest`
  commands. Note that `/retest` will only trigger failed PipelineRuns. This action takes precedence over the `pull_request` action.

## Configuring Policies in the Repository CR

To set up policies in the Repository CR, include the following configuration:

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
