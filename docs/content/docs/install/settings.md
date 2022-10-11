---
title: Settings
weight: 3
---

## Pipelines-As-Code configuration settings

There is a few things you can configure through the configmap
`pipelines-as-code` in the `pipelines-as-code` namespace.

* `application-name`

  The name of the application showing for example in the GitHub Checks
  labels. Default to `Pipelines as Code CI`

* `secret-auto-create`

  Whether to auto create a secret with the token generated through the GitHub
  application to be used with private repositories. This feature is enabled by
  default.

* `remote-tasks`

  Let allows remote tasks from pipelinerun annotations. This feature is enabled by
  default.

* `hub-url`

  The base URL for the [tekton hub](https://github.com/tektoncd/hub/)
  API. default to the [public hub](https://hub.tekton.dev/): <https://api.hub.tekton.dev/v1>

* `hub-catalog-name`

  The [tekton hub](https://github.com/tektoncd/hub/) catalog name. default to tekton

* `bitbucket-cloud-check-source-ip`

  Public bitbucket doesn't have the concept of Secret, we need to be
  able to secure the request by querying
  [atlassian ip ranges](https://ip-ranges.atlassian.com/),
  this only happen for public bitbucket (ie: when provider URL is not set in
  repository spec). If you want to override this, you need to bear in mind
  this could be a security issue, a malicious user can send a PR to your repo
  with a modification to your PipelineRun that would grab secrets, tunnel or
  others and then send a malicious webhook payload to the controller which
  look like a authorized owner has send the PR to run it.
  This feature is enabled by default.

* `bitbucket-cloud-additional-source-ip`

  This will provide us to give extra IPS (ie: 127.0.0.1) or networks (127.0.0.0/16)
  separated by commas.

* `max-keep-run-upper-limit`

  This let the user define a max limit for the max-keep-run value. When the user has defined a max-keep-run annotation
  on a pipelineRun then its value should be less than or equal to the upper limit, otherwise upper limit will be used for cleanup.

* `default-max-keep-runs`

  This allows user to define a default limit for max-keep-run value. If defined then it's applied to all the pipelineRun
  which do not have `max-keep-runs` annotation.
