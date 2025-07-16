---
title: Settings
weight: 3
---

## Pipelines-as-Code configuration settings

There are a few things you can configure through the ConfigMap
`pipelines-as-code` in the `pipelines-as-code` namespace.

* `application-name`

  The name of the application, for example when showing the results of the
  PipelineRun. If you're using the GitHub App, you will
  need to customize the label in the GitHub app settings as well. Defaults to
  `Pipelines-as-Code CI`.

* `secret-auto-create`

  Whether to auto-create a secret with the token generated through the GitHub
  application to be used with private repositories. This feature is enabled by
  default.

* `secret-github-app-token-scoped`

  When using a GitHub app, Pipelines-as-Code will generate a temporary
  installation token for every allowed event coming through the controller.

  The generated token will be scoped to the repository from where
  the payload comes from and not to every repository the app installation has
  access to.

  Having access to every repository the app has access to is a problem when
  you install the Pipelines-as-Code application into an organization that has
  a mix of public and private repositories where not every user in the
  organization is trusted to have access to the private repositories. Since
  the scoping of the token only allows the user to do operations and access on the
  repository where the payload comes from, it will not be able to access the private repositories.

  However, if you trust every user in your organization to access any repository or
  you are not planning to install your GitHub app globally on a GitHub
  organization, then you can safely set this option to false.

* `secret-github-app-scope-extra-repos`

  If you don't want to completely disable the scoping of the token, but still
  want some other repositories available (as long as you have installed the GitHub app on
  it), then you can add an extra owner/repo here.

  This lets you fetch remote URLs on GitHub from extra private repositories
  in an organization if you need it.

  This only works when all the repositories are added from the same installation IDs.

  You can have multiple owner/repository separated by commas:

  ```yaml
  secret-github-app-token-scoped: "owner/private-repo1, org/repo2"
  ```

* `remote-tasks`

  This allows fetching remote tasks on PipelineRun annotations. This feature is
  enabled by default.

* `bitbucket-cloud-check-source-ip`

  Public Bitbucket doesn't have the concept of Secret; we need to be
  able to secure the request by querying
  [Atlassian IP ranges](https://ip-ranges.atlassian.com/),
  this only happens for public Bitbucket (i.e., when provider URL is not set in
  repository spec). If you want to override this, you need to bear in mind
  this could be a security issue. A malicious user can send a PR to your repository
  with a modification to your PipelineRun that would grab secrets, tunnel or
  others and then send a malicious webhook payload to the controller which
  looks like an authorized owner has sent the PR to run it.
  This feature is enabled by default.

* `bitbucket-cloud-additional-source-ip`

  Let you add extra IPs to allow Bitbucket clouds. You can do a specific IP:
  `127.0.0.1` or a network `127.0.0.0/16`. Multiple IPs can be specified
  separated by commas.

* `max-keep-run-upper-limit`

  This lets the user define a max limit for the max-keep-run value. When the user
  has defined a max-keep-run annotation on a PipelineRun, then its value should
  be less than or equal to the upper limit; otherwise the upper limit will be used
  for cleanup.

* `default-max-keep-runs`

  This lets the user define a default limit for the `max-keep-run` value.
  When defined, it will be applied to all PipelineRuns without a `max-keep-runs`
  annotation.

* `auto-configure-new-github-repo`

  This setting lets you auto-configure newly created GitHub repositories. When
  Pipelines-as-Code sees a new repository URL from a payload, It will set
  up a namespace for your repository and create a Repository CR.

  This feature is disabled by default and is only supported with GitHub App.

{{< hint info >}}
 If you have a GitHub App already setup then verify if the `repository` event is
 subscribed into your GitHub App setting.
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

* `auto-configure-repo-repository-template`

  If `auto-configure-new-github-repo` is enabled then you can provide a template
  for generating the name for your new repository custom resource. By default, the repository custom resource name will be generated using this format `{{repo_name}}-repo-cr`.

  You can override the default using the following variables

  * `{{repo_owner}}`: The repository owner.
  * `{{repo_name}}`: The repository name.
  For example, if the template is defined as `{{repo_owner}}-{{repo_name}}-repo-cr`,
  then the Repository CR name generated for the repository
  `https://github.com/owner/test` will be `owner-test-repo-cr`

* `remember-ok-to-test`

  If `remember-ok-to-test` is true then if `ok-to-test` is done on pull request then in
  case of push event on pull request either through new commit or amend, then CI will
  re-run automatically

  By default, the `remember-ok-to-test` setting is set to false in Pipelines-as-Code to mitigate serious security risks.
  An attacker could submit a seemingly harmless PR to gain the repository owner's trust, and later
  inject malicious code designed to compromise the build system, such as exfiltrating secrets.

  Enabling this feature increases the risk of unauthorized access and is therefore strongly discouraged
  unless absolutely necessary. If you choose to enable it you can set it to true, you do so at your own
  risk and should be aware of the potential security vulnerabilities.
  (only GitHub and Gitea is supported at the moment).

* `skip-push-event-for-pr-commits`

  When enabled, this option prevents duplicate PipelineRuns when a commit appears in
  both a push event and a pull request. If a push event comes from a commit that is
  part of an open pull request, the push event will be skipped as it would create
  a duplicate PipelineRun.
  
  This feature works by checking if a pushed commit SHA exists in any open pull request,
  and if so, skipping the push event processing.

  Default: `true`

{{< support_matrix github_app="true" github_webhook="true" gitea="false" gitlab="false" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

### Global Cancel In Progress Settings

* `enable-cancel-in-progress-on-pull-requests`

  If the `enable-cancel-in-progress-on-pull-requests` setting is enabled (true), Pipelines-as-Code will automatically cancel
  any in-progress PipelineRuns associated with a pull request when a new update (such as a new commit) is pushed to that pull request.
  This ensures that only the latest commit is processed, helping conserve compute resources and avoid running outdated PipelineRuns
  tied to previous commits.

  It's important to note that if this global setting is disabled (false), Pipelines-as-Code will still honor the cancel-in-progress annotation
  at the individual PipelineRun level. In such cases, if a PipelineRun includes this annotation, it will take precedence over the global setting,
  and Pipelines-as-Code will cancel any matching in-progress runs when the pull request is updated.

  This is disabled by default.

* `enable-cancel-in-progress-on-push`

  If the `enable-cancel-in-progress-on-push` setting is enabled (true), Pipelines-as-Code will automatically cancel any in-progress PipelineRuns
  triggered by a push event when a new push is made to the same branch. This helps ensure that only the most recent commit is processed, preventing unnecessary execution of outdated PipelineRuns and optimizing resource usage.

  Additionally, if this global setting is disabled (false), Pipelines-as-Code will still respect the cancel-in-progress annotation
  on individual PipelineRuns. In such cases, the annotation will override the global configuration, and Pipelines-as-Code will
  cancel any in-progress runs for that specific PipelineRun when a new push occurs on the same branch.

  This is disabled by default.

### Remote Hub Catalogs

Pipelines-as-Code supports fetching tasks and pipelines with its remote annotations feature. By default, it will fetch from [Artifact Hub](https://artifacthub.io/), but you can also implicitly use the [Tekton Hub](https://hub.tekton.dev/) by using the `tektonhub://` prefix (as documented below) or point to your own custom hub with these settings:

* `hub-url`

  The base URL for the hub API. For Artifact Hub (default), this is set to <https://artifacthub.io>. For Tekton Hub, it would be <https://api.hub.tekton.dev/v1>.

* `hub-catalog-name`

  The catalog name in the hub. For Artifact Hub, the defaults are `tekton-catalog-tasks` for tasks and `tekton-catalog-pipelines` for pipelines. For Tekton Hub, this defaults to `tekton`.

* `hub-catalog-type`

  The type of hub catalog. Supported values are:
  
  * `artifacthub` - For Artifact Hub (default if not specified)
  * `tektonhub` - For Tekton Hub

* By default, both Artifact Hub and Tekton Hub are configured:
  
  * Artifact Hub is the default catalog (no prefix needed, but `artifact://` can be used explicitly)
  * Tekton Hub is available using the `tektonhub://` prefix

* Additionally you can have multiple hubs configured by using the following format:

  ```yaml
  catalog-1-id: "custom"
  catalog-1-name: "tekton"
  catalog-1-url: "https://api.custom.hub/v1"
  catalog-1-type: "tektonhub"
  
  catalog-2-id: "artifact"
  catalog-2-name: "tekton-catalog-tasks"
  catalog-2-url: "https://artifacthub.io"
  catalog-2-type: "artifacthub"
  ```

  Users are able to reference the custom hub by adding a prefix matching the catalog ID, such as `custom://` for a task they want to fetch from the `custom` catalog.

  You can add as many custom hubs as you want by incrementing the `catalog-NUMBER` number.

  Pipelines-as-Code will not try to fallback to the default or another custom hub
  if the task referenced is not found (the Pull Request will be set as failed)

### Error Detection

Pipelines-as-Code detect if the PipelineRun has failed and show a snippet of
the last few lines of the error.

When using the GitHub App, It will try to detect and match the error messages
in the container logs and expose them as [GitHub
annotations](https://github.blog/2018-12-14-introducing-check-runs-and-annotations/)
on Pull Request.

A few settings are available to configure this feature:

* `error-log-snippet`

  Enable or disable the feature to show a log snippet of the failed task when
  there is an error in a PipelineRun.

  Due to the constraints of the different GIT provider APIs, it will show a
  configurable number of lines of the first container from the first task that
  has exited with an error in the PipelineRun. The number of lines is controlled
  by the `error-log-snippet-number-of-lines` setting (see below).

  If it finds any strings matching the values of secrets attached to the
  PipelineRun it will replace it with the placeholder `******`

* `error-log-snippet-number-of-lines`

  default: `3`

  How many lines to show in the error log snippets when `error-log-snippet` is set to `"true"`.
  When using GitHub APP the GitHub Check interface [has a limit of 65535 characters](https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run),
  so you may want to be conservative with this setting.

* `error-detection-from-container-logs`

  Enable or disable the inspection of the container logs to detect error message
  and expose them as annotations on Pull Request.

  Only GitHub apps is supported.

* `error-detection-max-number-of-lines`

  How many lines to grab from the container when inspecting the
  logs for error detection when using `error-log-snippet`. Increasing this value
  may increase the watcher memory usage. Use `-1` for unlimited line to look error for.

* `error-detection-simple-regexp`

   By default the error detection only support a simple output, the way GCC or
   Make will output error, which is supported by most linters and command line tools.

   An example of an error that is supported is :

   ```console
   test.js:100:10: an error occurred
   ```

   Pipelines-as-Code will see this line and show it as an annotation on the pull
   request where the error occurred, in the `test.js` file at line 100.

   You can configure the default regexp used for detection. You will need to
   use regexp groups to pass the information of where the error occur, the regexp groups
   are:

   `<filename>`, `<line>`, `<error>`

### Reporting logs

  Pipelines-as-Code can report the logs of the tasks to the [OpenShift
  Console](https://docs.openshift.com/container-platform/latest/web_console/web-console.html),
  the [Tekton Dashboard](https://tekton.dev/docs/dashboard/) or if you have your
  own give you flexibility to link to your custom Dashboard.

#### OpenShift Console

  Pipelines-as-Code will automatically detect the OpenShift Console and link the logs of the tasks to the
  public URL of the OpenShift Console.

#### [Tekton Dashboard](https://tekton.dev/docs/dashboard/)

  If you are using the Tekton Dashboard, you can configure this feature using the
  `tekton-dashboard-url` setting. Simply set this to your dashboard URL, and the pipelinerun status and tasklog will be
  displayed there.

#### Custom Console (or dashboard)

  Alternatively, you have the ability to configure the links to go to your custom
  dashboard using the following settings:

* `custom-console-name`

  Set this to the name of your custom console. example: `MyCorp Console`

* `custom-console-url`

  Set this to the root URL of your custom console. example: `https://mycorp.com`

* `custom-console-url-namespace`

  Set this to the URL where to view the details of the `Namespace`.

  The URL supports all the standard variables as exposed on the PipelineRun (refer to
  the documentation on [Authoring PipelineRuns](../authoringprs)) with the added
  variable:

  * `{{ namespace }}`: The target namespace where the PipelineRun is executed

  example: `https://mycorp.com/ns/{{ namespace }}`

* `custom-console-url-pr-details`

  Set this to the URL where to view the details of the `PipelineRun`. This is
  shown when the PipelineRun is started so the user can follow execution on your
  console or when to see more details about the PipelineRun on result.

  The URL supports all the standard variables as exposed on the PipelineRun (refer to
  the documentation on [Authoring PipelineRuns](../authoringprs)) with the added
  variable:

  * `{{ namespace }}`: The target namespace where the PipelineRun is executed
  * `{{ pr }}`: The PipelineRun name.

  example: `https://mycorp.com/ns/{{ namespace }}/pipelinerun/{{ pr }}`

  Moreover it can access the [custom parameters](../guide/repositorycrd/#custom-parameter-expansion) from a
  Repository CR. For example if the user has a parameter in their Repo CR like this :

   ```yaml
   [...]
   spec:
    params:
      - name: custom
        value: value
   ```

  and the global configuration setting for `custom-console-url-pr-details` is:

  `https://mycorp.com/ns/{{ namespace }}/{{ custom }}`

  the `{{ custom }}` tag in the URL is expanded as `value`.

  This lets operators add specific information such as a `UUID` about a user as
  parameter in their repo CR and let it link to the console.

* `custom-console-url-pr-tasklog`

  Set this to the URL where to view the log of the taskrun of the `PipelineRun`. This is
  shown when we post a result of the task breakdown to link to the logs of the taskrun.

  The URL supports custom parameter on Repo CR and the standard parameters as
  described in the `custom-console-url-pr-details` setting and as well those added
  values:

  * `{{ namespace }}`: The target namespace where the PipelineRun is executed
  * `{{ pr }}`: The PipelineRun name.
  * `{{ task }}`: The Task name in the PR
  * `{{ pod }}`: The Pod name of the TaskRun
  * `{{ firstFailedStep }}`: The name of the first failed step in the TaskRun

  example: `https://mycorp.com/ns/{{ namespace }}/pipelinerun/{{ pr }}/logs/{{ task }}#{{ pod }}-{{ firstFailedStep }}`

## Pipelines-as-Code Info

  There are settings exposed through a ConfigMap for which any authenticated
  user can access to know about the pipelines-as-code status. This ConfigMap
  will be automatically created with the [OpenShift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/understanding-openshift-pipelines.html)
  or when installing with [tkn pac bootstrap](../../guide/cli/#bootstrap) command.

* `version`

  The version of Pipelines As Code currently installed.

* `controller-url`

  The controller URL as set by the `tkn pac bootstrap` command while setting up
  the GitHub App or if Pipelines-as-Code is installed

  The OpenShift Pipelines Operator will automatically set the route created
  for the controller.

  This field is also used to detect the controller URL when using the `tkn pac webhook add`
  commands.

* `provider`

  The provider set to `GitHub App` by tkn pac bootstrap, used to detect if a
  GitHub App is already configured when a user runs the bootstrap command a
  second time or the `webhook add` command.
