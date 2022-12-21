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

* `secret-github-app-token-scoped`

  When using a Github app, `Pipelines as Code` will generate a temporary
  installation token for every allowed event coming through the controller.

  The generated token will be scoped to the repository from the repository where
  the payload come from and not to every repositories the app installation has
  access to.

  Having access to every repositories the app has access to is a problem when
  you install the `Pipelines as Code` application into an organization that has
  a mixed between public and private repositories where every users in the
  organization is not trusted to have access to the private repositores. Since
  the scoping of the token only allow the user do operations and access on the
  repository where the payload come from, it will not be able to access the private repos.

  However, if you trust every users of your organization to access any repositories or
  you are not planning to install your GitHub app globally on a GitHub
  organization, then you can safely set this option to false.

* `secret-github-app-scope-extra-repos`

  If you don't want to completely disable the scoping of the token, but still
  wants some other repos available (as long you have installed the github app on
  it), then you can add an extra owner/repo here.

  This let you able fetch remote url on github from extra private repositories
  in an organisation if you need it.

  This only works when all the repos are added from the same installation IDs.

  You can have multiple owner/repository separated by commas:

  ```yaml
  secret-github-app-token-scoped: "owner/private-repo1, org/repo2"
  ```

* `remote-tasks`

  This allows fetching remote tasks on pipelinerun annotations. This feature is
  enabled by default.

* `hub-url`

  The base URL for the [tekton hub](https://github.com/tektoncd/hub/)
  API. This default to the [public hub](https://hub.tekton.dev/): <https://api.hub.tekton.dev/v1>

* `hub-catalog-name`

  The [tekton hub](https://github.com/tektoncd/hub/) catalog name. default to `tekton`

* `tekton-dashboard-url`

   When you are not running on Openshift using the [tekton
   dashboard](https://github.com/tektoncd/dashboard/) you will need to specify a
   dashboard url to have the logs tnd the pipelinerun details linked.

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

  Let you add extra IPS to allow bitbucket clouds, you can do a specific IP:
  `127.0.0.1` or a networks `127.0.0.0/16`. Multile of them can be specified
  separated by commas.

* `max-keep-run-upper-limit`

  This let the user define a max limit for the max-keep-run value. When the user
  has defined a max-keep-run annotation on a pipelineRun then its value should
  be less than or equal to the upper limit, otherwise upper limit will be used
  for cleanup.

* `default-max-keep-runs`

  This let the user define a default limit for the `max-keep-run` value.
  When defined it will applied to all the pipelineRun without a `max-keep-runs`
  annotation.

* `auto-configure-new-github-repo`

  This setting let you autoconfigure newly created GitHub repositories. When
  Pipelines as Code sees a new repository URL from a payload, It Code will set
  up a namespace for your repository and create a Repository CR.

  This feature is disabled by default and is only supported with GitHub App.

{{< hint info >}}
 If you have a GitHub App already setup then verify if the `repository` event is
 subscribed into your Github App setting.
{{< /hint >}}

* `auto-configure-repo-namespace-template`

  If `auto-configure-new-github-repo` is enabled then you can provide a template
  for generating the namespace for your new repository. By default, the
  namespace will be generated using this format `{{repo_name}}-pipelines`.

  You can override the default using the following variables

  * `{{repo_owner}}`: The repository owner.
  * `{{repo_name}}`: The repository name.

  For example. if the template is defined as `{{repo_owner}}-{{repo_name}}-ci`,
  then the namespace generated for repository

  `https://github.com/owner/repo` will be `owner-repo-ci`

* `error-log-snippet`

  Enable or disable the feature to show a log snippet of the failed task when
  there is an error in a PipelineRun.

  Due of the constraint of the different GIT provider API, It will show the last
  3 lines of the first container from the first task that has exited with an
  error in the PipelineRun.

  If it find any strings matching the values of secrets attached to the
  PipelineRun it will replace it with the placeholder `******`

* `error-log-snippet`

{{ hint danger }}
  alpha feature: may change at any time
{{ /hint danger }}

  Enable or disable the inspection of container logs to detect error message
  and expose them as annotations on Pull Request. Only Github apps is supported.

* `error-detection-max-number-of-lines`

{{ hint danger }}
  alpha feature: may change at any time
{{ /hint danger }}

  How many lines to grab from the container when inspecting the
  logs for error detection when using `error-log-snippet`. Increasing this value
  may increase the watcher memory usage. The default is 50, increase this value
  or use -1 for unlimited.

* `error-detection-simple-regexp`

{{ hint danger }}
  alpha feature: may change at any time
{{ /hint danger }}

   By default error detection only support the simple outputs, the way GCC or
   make will output which is supported by most linters and command line tools.

   An example is :

   ```console
   test.js:100:10: an error occurred
   ```

   Pipelines as Code will see this line and show it as an annotation on the pull
   request where the error occurred.

   You can configure the default regexp used for detection. You will need to
   keep the regexp groups: `<filename>`, `<line>`, `<error>` to make it works.

## Pipelines-As-Code Info

  There are a settings exposed through a config map for which any authenticated
  user can access to know about the Pipeline as Code status.

* `version`

  The version of Pipelines As Code currently installed.

* `controller-url`

  The controller URL as set by the `tkn pac bootstrap` command while setting up
  the GitHub App or if Pipelines as code is installed

  When using OpenShift Pipelines Operator then the operator sets the route created
  for the controller.

  This field is also used to detect the controller URL when using the `webhook add`
  commands.

* `provider`

  The provider set to `GitHub App` by tkn pac bootstrap, used to detect if a
  GitHub App is already configured when a user runs the bootstrap command a
  second time or the `webhook add` command.
