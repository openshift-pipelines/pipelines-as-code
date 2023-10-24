---
title: Policy on actions
weight: 50
---
# Policy on Pipelines-as-Code actions

Pipelines-as-Code has the concepts of Policy to let you control an action allowed
to be executed by a set of users belonging to a Team on an Organisation as
defined on GitHub or other Git Providers (only GitHub and Gitea is supported at
the moment).

## List of actions supported

* `pull_request` - This action is triggering the CI on Pipelines-as-Code,
   specifying a team will only allow the members of the team to trigger the CI
   and will not allow other members regadless if they are Owners or Collaborators
   of the repository or the Organization. The OWNERS file is still taken into
   account and will as well allow the members of the OWNERS file to trigger the
   CI.
* `ok_to_test` - This action will let a user belonging to the allowed team to
   issue a `/ok-to-test` comment on a Pull Request to trigger the CI on
   Pipelines-as-Code, this let running the CI on Pull Request contributed by a
   non collaborator of the repository or the organisation. This apply to the
   `/test` and `/retest` commands as well. This take precedence on the
   `pull_request` action.

## Configuring the Policy on the Repository CR

To configure the Policy on the Repository CR you need to add the following to the setting of the Repository CR:

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

Users in `ci-admins` team will be able to let other users run the CI on the pull
request and users in `ci-users` team will be able to run the CI on their own
pull request.

## Configuring teams on GitHub

You will need to configure the GitHub Apps on your organisation to use this
feature.

See the documentation on GitHub to configure the teams:

<https://docs.github.com/en/organizations/organizing-members-into-teams/about-teams>

## Configuring teams on Gitea

Teams on Gitea are configured on the Organization level. No documentation is
available but you can look at the GitHub documentation to get an idea of how to
configure it.
