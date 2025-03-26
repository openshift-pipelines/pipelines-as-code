---
title: Settings
weight: 3
---

## Pipelines-as-Code configuration settings

There is a few things you can configure through the config map
`pipelines-as-code` in the `pipelines-as-code` namespace.

* `application-name`

  The name of the application for example when showing the results of the
  pipelinerun. If you're using the GitHub App you will
  need to customize the label on the github app setting as well. .  Default to
  `Pipelines-as-Code CI`

* `secret-auto-create`

  Whether to auto create a secret with the token generated through the GitHub
  application to be used with private repositories. This feature is enabled by
  default.

* `secret-github-app-token-scoped`

  When using a GitHub app, `Pipelines-as-Code` will generate a temporary
  installation token for every allowed event coming through the controller.

  The generated token will be scoped to the repository from the repository where
  the payload come from and not to every repositories the app installation has
  access to.

  Having access to every repositories the app has access to is a problem when
  you install the `Pipelines-as-Code` application into an organization that has
  a mixed between public and private repositories where every users in the
  organization is not trusted to have access to the private repositories. Since
  the scoping of the token only allow the user do operations and access on the
  repository where the payload come from, it will not be able to access the private repos.

  However, if you trust every users of your organization to access any repositories or
  you are not planning to install your GitHub app globally on a GitHub
  organization, then you can safely set this option to false.

* `secret-github-app-scope-extra-repos`

  If you don't want to completely disable the scoping of the token, but still
  wants some other repos available (as long you have installed the GitHub app on
  it), then you can add an extra owner/repo here.

  This let you able fetch remote url on GitHub from extra private repositories
  in an organisation if you need it.

  This only works when all the repos are added from the same installation IDs.

  You can have multiple owner/repository separated by commas:

  ```yaml
  secret-github-app-token-scoped: "owner/private-repo1, org/repo2"
  ```

* `remote-tasks`

  This allows fetching remote tasks on pipelinerun annotations. This feature is
  enabled by default.

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
  `127.0.0.1` or a networks `127.0.0.0/16`. Multiple of them can be specified
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
  Pipelines-as-Code sees a new repository URL from a payload, It Code will set
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

### Tekton Hub support

Pipelines-as-Code supports fetching task with its remote annotations feature, by default it will fetch it from the [public tekton hub](https://hub.tekton.dev/) but you can configure it to point to your own with these settings:

* `hub-url`

  The base URL for the [tekton hub](https://github.com/tektoncd/hub/)
  API. This default to the [public hub](https://hub.tekton.dev/): <https://api.hub.tekton.dev/v1>

* `hub-catalog-name`

  The [tekton hub](https://github.com/tektoncd/hub/) catalog name. default to `tekton`

* Additionally you can have multiple hub configured by using the following format:

  ```yaml
  catalog-1-id: "custom"
  catalog-1-name: "tekton"
  catalog-1-url: "https://api.custom.hub/v1"
  ```

  Users are able to reference the custom hub by adding a `custom://` prefix to
  their task they want to fetch from the `custom` catalog.

  You can add as many custom hub as you want by incrementing the `catalog-NUMBER` number.

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

  Due of the constraint of the different GIT provider API, It will show the last
  3 lines of the first container from the first task that has exited with an
  error in the PipelineRun.

  If it find any strings matching the values of secrets attached to the
  PipelineRun it will replace it with the placeholder `******`

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

  The URL supports all the standard variables as exposed on the Pipelinerun (refer to
  the documentation on [Authoring PipelineRuns](../authoringprs)) with the added
  variable:

  * `{{ namespace }}`: The target namespace where the pipelinerun is executed

  example: `https://mycorp.com/ns/{{ namespace }}`

* `custom-console-url-pr-details`

  Set this to the URL where to view the details of the `PipelineRun`. This is
  shown when the PipelineRun is started so the user can follow execution on your
  console or when to see more details about the pipelinerun on result.

  The URL supports all the standard variables as exposed on the Pipelinerun (refer to
  the documentation on [Authoring PipelineRuns](../authoringprs)) with the added
  variable:

  * `{{ namespace }}`: The target namespace where the pipelinerun is executed
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

  This let operator to add specific information like a `UUID` about a user as
  parameter in their repo CR and let it link to the console.

* `custom-console-url-pr-tasklog`

  Set this to the URL where to view the log of the taskrun of the `PipelineRun`. This is
  shown when we post a result of the task breakdown to link to the logs of the taskrun.

  The URL supports custom parameter on Repo CR and the standard parameters as
  described in the `custom-console-url-pr-details` setting and as well those added
  values:

  * `{{ namespace }}`: The target namespace where the pipelinerun is executed
  * `{{ pr }}`: The PipelineRun name.
  * `{{ task }}`: The Task name in the PR
  * `{{ pod }}`: The Pod name of the TaskRun
  * `{{ firstFailedStep }}`: The name of the first failed step in the TaskRun

  example: `https://mycorp.com/ns/{{ namespace }}/pipelinerun/{{ pr }}/logs/{{ task }}#{{ pod }}-{{ firstFailedStep }}`

## Pipelines-as-Code Info

  There are a settings exposed through a config map for which any authenticated
  user can access to know about the Pipelines-as-Code status. This Configmap
  will be automatically created with the [OpenShift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/understanding-openshift-pipelines.html)
  or when installing with [tkn pac bootstrap](../../guide/cli/#bootstrap) command.

* `version`

  The version of Pipelines As Code currently installed.

* `controller-url`

  The controller URL as set by the `tkn pac bootstrap` command while setting up
  the GitHub App or if Pipelines as code is installed

  The OpenShift Pipelines Operator will automatically set the the route created
  for the controller.

  This field is also used to detect the controller URL when using the `tkn pac webhook add`
  commands.

* `provider`

  The provider set to `GitHub App` by tkn pac bootstrap, used to detect if a
  GitHub App is already configured when a user runs the bootstrap command a
  second time or the `webhook add` command.

## Logging Configuration

  Pipelines-as-Code uses the ConfigMap named `pac-config-logging` in the same namespace (`pipelines-as-code` by default) as the controllers. To get the ConfigMap use the following command:

  ```bash
  $ kubectl get configmap pac-config-logging -n pipelines-as-code

  NAME                 DATA   AGE
  pac-config-logging   4      9m44s
  ```

  To retrieve the content of the ConfigMap:

  ```bash
  $ kubectl get configmap pac-config-logging -n pipelines-as-code -o yaml

  apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      app.kubernetes.io/instance: default
      app.kubernetes.io/part-of: pipelines-as-code
    name: pac-config-logging
    namespace: pipelines-as-code
  data:
    loglevel.pac-watcher: info
    loglevel.pipelines-as-code-webhook: info
    loglevel.pipelinesascode: info
    zap-logger-config: |
      {
        "level": "info",
        "development": false,
        "sampling": {
          "initial": 100,
          "thereafter": 100
        },
        "outputPaths": ["stdout"],
        "errorOutputPaths": ["stderr"],
        "encoding": "json",
        "encoderConfig": {
          "timeKey": "ts",
          "levelKey": "level",
          "nameKey": "logger",
          "callerKey": "caller",
          "messageKey": "msg",
          "stacktraceKey": "stacktrace",
          "lineEnding": "",
          "levelEncoder": "",
          "timeEncoder": "iso8601",
          "durationEncoder": "",
          "callerEncoder": ""
        }
      }
  ```

  The `loglevel.*` fields define the log level for the controllers:

* loglevel.pipelinesascode - the log level for the pipelines-as-code-controller component
* loglevel.pipelines-as-code-webhook - the log level for the pipelines-as-code-webhook component
* loglevel.pac-watcher - the log level for the pipelines-as-code-watcher component

  You can change the log level from `info` to `debug` or any other supported values. For example, select the `debug` log level for the pipelines-as-code-watcher component:

  ```bash
  kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "replace", "path": "/data/loglevel.pac-watcher", "value":"debug"}]'
  ```

  After this command, the controller gets a new log level value.
  If you want to use the same log level for all Pipelines-as-Code components, delete `level.*` values from configmap:

  ```bash
  kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[  {"op": "remove", "path": "/data/loglevel.pac-watcher"},  {"op": "remove", "path": "/data/loglevel.pipelines-as-code-webhook"},  {"op": "remove", "path": "/data/loglevel.pipelinesascode"}]'
  ```

  In this case, all Pipelines-as-Code components get a common log level from `zap-logger-config` - `level` field from the json.

  `zap-logger-config` supports the following log levels:

* debug - fine-grained debugging
* info - normal logging
* warn - unexpected but non-critical errors
* error - critical errors; unexpected during normal operation
* dpanic - in debug mode, trigger a panic (crash)
* panic - trigger a panic (crash)
* fatal - immediately exit with exit status 1 (failure)

  See more: <https://knative.dev/docs/serving/observability/logging/config-logging>
