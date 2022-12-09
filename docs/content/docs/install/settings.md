---
title: Settings
weight: 3
---

## Pipelines-As-Code configuration settings

There is a few things you can configure through the config map
`pipelines-as-code` in the `pipelines-as-code` namespace.

* `application-name`

  The name of the application showing for example in the GitHub Checks
  labels. Default to `Pipelines as Code CI`

* `secret-auto-create`

  Whether to auto create a secret with the token generated through the GitHub
  application to be used with private repositories. This feature is enabled by
  default.

* `secret-github-apps-token-scopped`

  When using a Github app, we generate a temporary installation token, we scope it
  to the repository from where the payload comes. We do this when the Github app
  is configured globally on a Github organization.

  If the organization has a mix of public and private repositories and not every
  user in the organization is trusted to have access to every repository, then the
  scoped token would not allow them to access those.

  If you trust every user on your organization to access any repository or you are
  not planning to install your Github app globally on a Github organization, then
  you can safely set this option to false.

* `remote-tasks`

  Let allows remote tasks from pipelinerun annotations. This feature is enabled by
  default.

* `hub-url`

  The base URL for the [tekton hub](https://github.com/tektoncd/hub/)
  API. default to the [public hub](https://hub.tekton.dev/): <https://api.hub.tekton.dev/v1>

* `hub-catalog-name`

  The [tekton hub](https://github.com/tektoncd/hub/) catalog name. default to tekton

* `tekton-dashboard-url`

  Using the URL of the Tekton dashboard, Pipelines-as-Code generates a URL to the PipelineRun on the Tekton dashboard.
  If you are an OpenShift user, then OpenShift console URL is auto-detected.

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

* `auto-configure-new-github-repo`

  This setting let you autoconfigure newly created GitHub repositories. On creation of a new repository, Pipelines As Code will set up a namespace
  for your repository and create a Repository CR.

  This feature is disabled by default and is only supported with GitHub App.

{< hint info >}
 If you have a GitHub App already setup then verify if `Repository` event is subscribed.
{< /hint >}

* `auto-configure-repo-namespace-template`

  If `auto-configure-new-github-repo` is enabled then you can provide a template for generating the namespace for your new repository.
  By default, the namespace will be generated using this format `{{repo_name}}-pipelines`.

  You can override the default using the following variables

  * `{{repo_owner}}`: The repository owner.
  * `{{repo_name}}`: The repository name.

  for example. if the template is defined as `{{repo_owner}}-{{repo_name}}-ci`, then the namespace generated for repository
  `https://github.com/owner/repo` will be `owner-repo-ci`

* `error-log-snippet`

  Enable or disable the feature to show a log snippet of the failed task when there is
  an error in a Pipeline

  It will show the last 3 lines of the first container of the first task
  that has error in the pipeline.

  If it find any strings matching the values of secrets attached to the PipelineRun it will replace it with the placeholder `******`

## Pipelines-As-Code Info

  There are a settings exposed through a config map which any authenticated user can access to know about
  Pipeline as Code.

* `version`

  The version of Pipelines As Code installed.

* `controller-url`

  The controller URL as set by the `tkn pac bootstrap` command while setting up the GitHub App or if Pipelines as code is installed
  using OpenShift Pipelines Operator then the operator sets the route created for the controller. This field is also used to detect the controller
  URL in `webhook add` commands.

* `provider`

  The provider is set to `GitHub App` by tkn pac bootstrap command and is used to detect if a GitHub App is already configured when a user runs the
  bootstrap command a second time or the `webhook add` command.
