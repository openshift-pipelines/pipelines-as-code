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

* `max-keep-days`

  The number of the day to keep the PipelineRuns runs in the `pipelines-as-code`
  namespace. We install by default a cronjob that cleans up the PipelineRuns
  generated on events in pipelines-as-code namespace. Note that these
  PipelineRuns are internal to Pipelines-as-code are separate from the
  PipelineRuns that exist in the user's GitHub repository. The cronjob runs
  every hour and by default cleanups PipelineRuns over a day. This configmap
  setting doesn't affect the cleanups of the user's PipelineRuns which are
  controlled by the [annotations on the PipelineRun definition in the user's
  GitHub repository](#pipelineruns-cleanups).

* `secret-auto-create`

  Whether to auto create a secret with the token generated through the GitHub
  application to be used with private repositories. This feature is enabled by
  default.

* `remote-tasks`

  Let allows remote tasks from pipelinerun annotations. This feature is enabled by
  default.

* `hub-url`

  The base URL for the [tekton hub](https://github.com/tektoncd/hub/)
  API. default to the [public hub](https://hub.tekton.dev/):

  <https://api.hub.tekton.dev/v1>
