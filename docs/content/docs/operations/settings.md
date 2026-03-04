---
title: Settings
weight: 7
---

This page provides a detailed reference for all Pipelines-as-Code settings available in the `pipelines-as-code` ConfigMap. Use it alongside the [Configuration]({{< relref "configuration" >}}) page when you need to understand every option in detail.

## Pipelines-as-Code Configuration Settings

You configure Pipelines-as-Code through the `pipelines-as-code` ConfigMap in the `pipelines-as-code` namespace.

* `application-name`

  The name of the application that appears in PipelineRun results. If you use a GitHub App, you must also update this label in your GitHub App settings. Defaults to `Pipelines-as-Code CI`.

* `secret-auto-create`

  Pipelines-as-Code automatically creates a secret containing the token generated through the GitHub application. Private repositories use this secret for authentication. This feature is enabled by default.

* `secret-github-app-token-scoped`

  When you use a GitHub App, Pipelines-as-Code generates a temporary installation token for every allowed event coming through the controller.

  Pipelines-as-Code scopes this token to the repository where the payload originated, rather than granting access to every repository the app installation can reach.

  This scoping matters when you install the Pipelines-as-Code GitHub App on an organization that has a mix of public and private repositories. Without scoping, any user triggering a PipelineRun could access private repositories they should not reach. With scoping enabled, the token only permits operations on the repository that generated the event.

  If you trust every user in your organization to access any repository, or you do not install the GitHub App at the organization level, you can safely set this option to `false`.

* `secret-github-app-scope-extra-repos`

  If you want to keep token scoping enabled but need access to additional repositories, list them here. This allows PipelineRuns to fetch remote resources from specific private repositories without disabling scoping entirely.

  All listed repositories must belong to the same GitHub App installation.

  Separate multiple owner/repository pairs with commas:

  ```yaml
  secret-github-app-token-scoped: "owner/private-repo1, org/repo2"
  ```

* `remote-tasks`

  Allows Pipelines-as-Code to fetch remote tasks referenced in PipelineRun annotations. This feature is enabled by default.

* `bitbucket-cloud-check-source-ip`

  Because public Bitbucket Cloud does not support webhook secrets, Pipelines-as-Code secures incoming requests by checking them against [Atlassian IP ranges](https://ip-ranges.atlassian.com/). This check only applies to public Bitbucket (when `provider.url` is not set in the Repository CR spec).

  Disabling this setting creates a security risk. A malicious user could send a crafted webhook payload to trigger a PipelineRun that exfiltrates secrets or runs unauthorized code. This feature is enabled by default.

* `bitbucket-cloud-additional-source-ip`

  Adds extra IPs or networks to the Bitbucket Cloud allow list. You can specify a single IP (`127.0.0.1`) or a CIDR range (`127.0.0.0/16`). Separate multiple values with commas.

* `max-keep-run-upper-limit`

  Sets the maximum allowed value for the `pipelinesascode.tekton.dev/max-keep-runs` annotation. If a user sets a `max-keep-runs` value higher than this limit, Pipelines-as-Code uses the upper limit for cleanup instead.

* `default-max-keep-runs`

  Sets a default limit for the `max-keep-runs` value. Pipelines-as-Code applies this default to every PipelineRun that does not carry a `max-keep-runs` annotation.

* `auto-configure-new-github-repo`

  Pipelines-as-Code automatically configures newly created GitHub repositories. When it detects a new repository URL in a webhook payload, it creates a namespace and a Repository CR for that repository.

  This feature is disabled by default and only works with GitHub Apps.

{{< callout type="info" >}}
 If you have a GitHub App already setup then verify if the `repository` event is
 subscribed into your GitHub App setting.
{{< /callout >}}

* `auto-configure-repo-namespace-template`

  When `auto-configure-new-github-repo` is enabled, this template controls the namespace name for newly configured repositories. By default, Pipelines-as-Code uses the format `{{repo_name}}-pipelines`.

  You can override the default with these variables:

  * `{{repo_owner}}`: The repository owner.
  * `{{repo_name}}`: The repository name.

  For example, if the template is `{{repo_owner}}-{{repo_name}}-ci`, then the namespace for `https://github.com/owner/repo` becomes `owner-repo-ci`.

* `auto-configure-repo-repository-template`

  When `auto-configure-new-github-repo` is enabled, this template controls the Repository CR name. By default, Pipelines-as-Code uses the format `{{repo_name}}-repo-cr`.

  You can override the default with these variables:

  * `{{repo_owner}}`: The repository owner.
  * `{{repo_name}}`: The repository name.
  For example, if the template is `{{repo_owner}}-{{repo_name}}-repo-cr`, then the Repository CR name for `https://github.com/owner/test` becomes `owner-test-repo-cr`.

* `remember-ok-to-test`

  When you enable this setting, Pipelines-as-Code automatically re-runs CI on pull request updates (new commits or amends) after the initial `/ok-to-test` approval, without requiring a new approval comment.

  By default, `remember-ok-to-test` is set to `false` to mitigate serious security risks. An attacker could submit a seemingly harmless pull request to gain trust, then inject malicious code in a later commit to compromise the build system and exfiltrate secrets.

  Enabling this feature increases the risk of unauthorized access. If you choose to enable it, you do so at your own risk and should understand the potential security vulnerabilities. Only GitHub and Forgejo support this feature at the moment.

* `skip-push-event-for-pr-commits`

  Prevents duplicate PipelineRuns when a commit appears in both a push event and a pull request. If a push event comes from a commit that belongs to an open pull request, Pipelines-as-Code skips the push event to avoid creating a duplicate PipelineRun.

  Pipelines-as-Code checks whether the pushed commit SHA exists in any open pull request and, if so, skips push event processing.

  Default: `true`

{{< callout type="info" >}}
This setting does not apply to git tag push events. Tag push events always trigger
pipeline runs regardless of whether the tagged commit is part of an open pull request.
{{< /callout >}}

{{< support_matrix github_app="true" github_webhook="true" forgejo="false" gitlab="false" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

### Global Cancel In Progress Settings

* `enable-cancel-in-progress-on-pull-requests`

  When you enable this setting, Pipelines-as-Code automatically cancels any in-progress PipelineRuns associated with a pull request when that pull request receives a new update (such as a new commit). This ensures only the latest commit runs, conserving compute resources and preventing outdated PipelineRuns from completing.

  Even when this global setting is disabled, Pipelines-as-Code still honors the `pipelinesascode.tekton.dev/on-cancel-in-progress` annotation on individual PipelineRuns. If a PipelineRun includes that annotation, it takes precedence over the global setting.

  This is disabled by default.

* `enable-cancel-in-progress-on-push`

  When you enable this setting, Pipelines-as-Code automatically cancels any in-progress PipelineRuns triggered by a push event when a new push occurs on the same branch. This ensures only the most recent commit runs, preventing unnecessary execution and optimizing resource usage.

  Even when this global setting is disabled, Pipelines-as-Code still respects the cancel-in-progress annotation on individual PipelineRuns. The annotation overrides the global configuration for that specific PipelineRun.

  This is disabled by default.

### Remote Hub Catalogs

Pipelines-as-Code can fetch tasks and pipelines from remote catalogs through its annotation-based remote task feature. By default, it fetches from [Artifact Hub](https://artifacthub.io/). You can also point it to your own custom hub using the settings below.

* `hub-url`

  The base URL for the hub API. Defaults to <https://artifacthub.io>.

* `hub-catalog-name`

  The catalog name in the hub. For Artifact Hub, the defaults are `tekton-catalog-tasks` for tasks and `tekton-catalog-pipelines` for pipelines.

* `hub-catalog-type`

  The type of hub catalog. Supported values:

  * `artifacthub` - For Artifact Hub (default)
  * `tektonhub` - For custom self-hosted Tekton Hub instances

  If `hub-catalog-type` is empty, Pipelines-as-Code auto-detects the catalog type by probing the Artifact Hub stats endpoint. If the endpoint responds successfully, Pipelines-as-Code treats it as Artifact Hub; otherwise, it falls back to Tekton Hub.

* You can also configure multiple hubs using numbered prefixes:

  ```yaml
  catalog-1-id: "custom"
  catalog-1-name: "tekton"
  catalog-1-url: "https://api.custom.hub/v1"
  catalog-1-type: "tektonhub"

  catalog-2-id: "artifact"
  catalog-2-name: "tekton-catalog-tasks"
  catalog-2-url: "https://artifacthub.io/api/v1"
  catalog-2-type: "artifacthub"
  ```

  You can reference a custom hub in your PipelineRun annotations by adding a prefix that matches the catalog ID, such as `custom://` to fetch a task from the `custom` catalog.

  Add as many custom hubs as you need by incrementing the `catalog-NUMBER` prefix.

  Pipelines-as-Code does not fall back to the default or another custom hub if it cannot find a referenced task. Instead, it marks the pull request as failed.

### Error Detection

Pipelines-as-Code detects when a PipelineRun fails and shows a snippet of the last few lines of the error in the pull request status.

When you use a GitHub App, Pipelines-as-Code also detects and matches error messages in container logs and surfaces them as [GitHub annotations](https://github.blog/2018-12-14-introducing-check-runs-and-annotations/) on the pull request.

The following settings control this behavior:

* `error-log-snippet`

  Enables or disables log snippets from the failed task when a PipelineRun errors out.

  Because of Git provider API constraints, Pipelines-as-Code shows a configurable number of lines from the first container of the first task that exited with an error. You control the number of lines through the `error-log-snippet-number-of-lines` setting (see below).

  If the log output contains strings matching any secret values attached to the PipelineRun, Pipelines-as-Code replaces them with the placeholder `******`.

* `error-log-snippet-number-of-lines`

  default: `3`

  The number of lines to include in error log snippets when `error-log-snippet` is `"true"`. When using a GitHub App, the GitHub Check interface [has a limit of 65535 characters](https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run), so keep this value conservative.

* `error-detection-from-container-logs`

  Enables or disables inspection of container logs to detect error messages and surface them as annotations on pull requests.

  Only GitHub Apps support this feature.

* `error-detection-max-number-of-lines`

  The number of lines Pipelines-as-Code reads from container logs when inspecting for error detection with `error-log-snippet`. Increasing this value may increase watcher memory usage. Set to `-1` for unlimited.

* `error-detection-simple-regexp`

   By default, error detection supports simple output formats like those produced by GCC, Make, and most linters and command line tools.

   An example of a supported error format:

   ```console
   test.js:100:10: an error occurred
   ```

   Pipelines-as-Code matches this line and shows it as an annotation on the pull request at the `test.js` file, line 100.

   You can customize the regexp used for detection. The regexp must include these named groups to pass location information:

   `<filename>`, `<line>`, `<error>`

### Reporting Logs

  Pipelines-as-Code can link task logs to the [OpenShift Console](https://docs.openshift.com/container-platform/latest/web_console/web-console.html), the [Tekton Dashboard](https://tekton.dev/docs/dashboard/), or a custom dashboard of your choice.

#### OpenShift Console

  Pipelines-as-Code automatically detects the OpenShift Console and links task logs to its public URL.

#### [Tekton Dashboard](https://tekton.dev/docs/dashboard/)

  If you use the Tekton Dashboard, set the `tekton-dashboard-url` setting to your dashboard URL. Pipelines-as-Code then links PipelineRun status and task logs to your dashboard.

#### Custom Console (or dashboard)

  You can also configure links pointing to your own custom dashboard using the following settings:

* `custom-console-name`

  The name of your custom console. Example: `MyCorp Console`

* `custom-console-url`

  The root URL of your custom console. Example: `https://mycorp.com`

* `custom-console-url-namespace`

  The URL template for viewing Namespace details.

  This URL supports all the standard variables exposed on the PipelineRun (refer to the documentation on [Authoring PipelineRuns]({{< relref "/docs/guides/creating-pipelines" >}})) with the added variable:

  * `{{ namespace }}`: The target namespace where the PipelineRun runs

  Example: `https://mycorp.com/ns/{{ namespace }}`

* `custom-console-url-pr-details`

  The URL template for viewing PipelineRun details. Pipelines-as-Code shows this link when a PipelineRun starts, so you can follow execution on your console or view results.

  This URL supports all the standard variables exposed on the PipelineRun (refer to the documentation on [Authoring PipelineRuns]({{< relref "/docs/guides/creating-pipelines" >}})) with these added variables:

  * `{{ namespace }}`: The target namespace where the PipelineRun runs
  * `{{ pr }}`: The PipelineRun name

  Example: `https://mycorp.com/ns/{{ namespace }}/pipelinerun/{{ pr }}`

  This URL can also access [custom parameters]({{< relref "/docs/advanced/custom-parameters" >}}) from a Repository CR. For example, if your Repository CR defines:

   ```yaml
   [...]
   spec:
    params:
      - name: custom
        value: value
   ```

  and you set `custom-console-url-pr-details` to:

  `https://mycorp.com/ns/{{ namespace }}/{{ custom }}`

  Pipelines-as-Code expands `{{ custom }}` to `value`.

  This lets operators add specific information such as a UUID about a user as a parameter in the Repository CR and link it to the console.

* `custom-console-url-pr-tasklog`

  The URL template for viewing TaskRun logs. Pipelines-as-Code shows this link in the task breakdown results to link to TaskRun logs.

  This URL supports custom parameters from the Repository CR and the standard parameters described in `custom-console-url-pr-details`, plus these additional values:

  * `{{ namespace }}`: The target namespace where the PipelineRun runs
  * `{{ pr }}`: The PipelineRun name
  * `{{ task }}`: The Task name in the PipelineRun
  * `{{ pod }}`: The Pod name of the TaskRun
  * `{{ firstFailedStep }}`: The name of the first failed step in the TaskRun

  Example: `https://mycorp.com/ns/{{ namespace }}/pipelinerun/{{ pr }}/logs/{{ task }}#{{ pod }}-{{ firstFailedStep }}`

## Pipelines-as-Code Info

  Pipelines-as-Code exposes status information through a ConfigMap that any authenticated user can read. The [OpenShift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/understanding-openshift-pipelines.html) creates this ConfigMap automatically, and it is also created when you run the [tkn pac bootstrap]({{< relref "/docs/cli" >}}) command.

* `version`

  The version of Pipelines-as-Code currently installed.

* `controller-url`

  The controller URL that `tkn pac bootstrap` sets while configuring the GitHub App. The OpenShift Pipelines Operator automatically sets this to the route created for the controller.

  Pipelines-as-Code also uses this field to detect the controller URL when you run the `tkn pac webhook add` command.

* `provider`

  The provider type (set to `GitHub App` by `tkn pac bootstrap`). Pipelines-as-Code uses this value to detect whether a GitHub App is already configured when you run the bootstrap command a second time or the `webhook add` command.
