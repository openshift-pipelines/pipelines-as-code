---
title: "Configuration"
weight: 1
---

This page describes all configuration options available in the Pipelines-as-Code ConfigMap. Use it as a reference when you need to change controller behavior, authentication settings, or error reporting.

Pipelines-as-Code stores its configuration in the `pipelines-as-code` ConfigMap in the `pipelines-as-code` namespace. This ConfigMap controls how the controller handles authentication, remote tasks, error detection, and more.

## Viewing the Configuration

To view the current configuration:

```bash
kubectl get configmap pipelines-as-code -n pipelines-as-code -o yaml
```

## Configuration Reference

All configuration options live in the `data` section of the ConfigMap. The sections below provide a complete reference for every available setting.

### Application Settings

## application-name {#param-application-name}

application-name

string

default:"Pipelines as Code CI"

The name of the application that appears in PipelineRun results. If you use a GitHub App, you must also update this label in your GitHub App settings.

### Authentication & Security

## secret-auto-create {#param-secret-auto-create}

secret-auto-create

boolean

default:"true"

Pipelines-as-Code automatically creates a secret containing the token generated through the GitHub application. This secret allows PipelineRuns to access private repositories.

## secret-github-app-token-scoped {#param-secret-github-app-token-scoped}

secret-github-app-token-scoped

boolean

default:"true"

Pipelines-as-Code scopes each GitHub App installation token to only the repository where the event originated, rather than granting access to every repository the app can reach. This protects organizations that have a mix of public and private repositories where not all users should access private repositories.

Setting `secret-github-app-token-scoped` to `false` grants the token access to all repositories where the GitHub App is installed. Only disable scoping if you trust every user in your organization.

## secret-github-app-scope-extra-repos {#param-secret-github-app-scope-extra-repos}

secret-github-app-scope-extra-repos

string

default:""

Adds specific repositories to the token scope without disabling scoping entirely. All listed repositories must belong to the same GitHub App installation. Example:

```yaml
secret-github-app-scope-extra-repos: "owner/private-repo1, org/repo2"
```

### Remote Tasks & Catalogs

## remote-tasks {#param-remote-tasks}

remote-tasks

boolean

default:"true"

Allows Pipelines-as-Code to fetch remote tasks referenced in PipelineRun annotations.

## hub-url {#param-hub-url}

hub-url

string

default:"<https://artifacthub.io>"

The base URL for the hub API that Pipelines-as-Code queries when fetching tasks and pipelines.

## hub-catalog-type {#param-hub-catalog-type}

hub-catalog-type

string

default:"artifacthub"

The type of hub catalog. Supported values:

- `artifacthub` - For Artifact Hub (default)
- `tektonhub` - For custom self-hosted Tekton Hub instances

### Additional Catalogs

You can configure multiple custom catalogs using numbered prefixes:

```yaml
catalog-1-id: custom
catalog-1-name: tekton
catalog-1-url: https://api.custom.hub/v1
catalog-1-type: tektonhub

catalog-2-id: artifact
catalog-2-name: tekton-catalog-tasks
catalog-2-url: https://artifacthub.io
catalog-2-type: artifacthub
```

You can then reference a custom catalog in your PipelineRun annotations by prefixing the task name with the catalog ID:

```yaml
pipelinesascode.tekton.dev/task: "custom://task-name"
```

### Error Detection & Reporting

## error-log-snippet {#param-error-log-snippet}

error-log-snippet

boolean

default:"true"

Displays a log snippet from the failed task when a PipelineRun fails. Disable this setting if your pipelines might leak sensitive values in logs.

## error-log-snippet-number-of-lines {#param-error-log-snippet-number-of-lines}

error-log-snippet-number-of-lines

integer

default:"3"

The number of lines to include in error log snippets. The GitHub Check interface has a 65,535 character limit, so keep this value conservative.

## error-detection-from-container-logs {#param-error-detection-from-container-logs}

error-detection-from-container-logs

boolean

default:"true"

Pipelines-as-Code inspects container logs to detect error messages and surfaces them as annotations on pull requests. Only GitHub Apps support this feature.

## error-detection-max-number-of-lines {#param-error-detection-max-number-of-lines}

error-detection-max-number-of-lines

integer

default:"50"

The maximum number of lines Pipelines-as-Code inspects from container logs for error detection. Set to `-1` for unlimited. Increasing this value may increase watcher memory usage.

## error-detection-simple-regexp {#param-error-detection-simple-regexp}

error-detection-simple-regexp

string

The regular expression Pipelines-as-Code uses for simple error detection. The regexp must include these named groups: `filename`, `line`, `column`, and `error`. The default pattern matches errors like: `test.js:100:10: an error occurred`

```text
^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+)?([ ]*)?(?P<error>.*)
```

### Pipeline Run Management

## enable-cancel-in-progress-on-pull-requests {#param-enable-cancel-in-progress-on-pull-requests}

enable-cancel-in-progress-on-pull-requests

boolean

default:"false"

Pipelines-as-Code automatically cancels in-progress PipelineRuns when a pull request receives a new update. This conserves resources by stopping outdated runs.

## enable-cancel-in-progress-on-push {#param-enable-cancel-in-progress-on-push}

enable-cancel-in-progress-on-push

boolean

default:"false"

Pipelines-as-Code automatically cancels in-progress PipelineRuns when a new push occurs on the same branch.

Individual PipelineRuns can override global cancel-in-progress settings using the `pipelinesascode.tekton.dev/on-cancel-in-progress` annotation.

## max-keep-run-upper-limit {#param-max-keep-run-upper-limit}

max-keep-run-upper-limit

integer

default:""

Sets the maximum value that users can assign to the `pipelinesascode.tekton.dev/max-keep-runs` annotation. If a user sets a higher value, Pipelines-as-Code applies this upper limit instead during cleanup.

## default-max-keep-runs {#param-default-max-keep-runs}

default-max-keep-runs

integer

default:""

Sets a default number of PipelineRuns to keep. Pipelines-as-Code applies this value to every PipelineRun that does not carry a `max-keep-runs` annotation.

## skip-push-event-for-pr-commits {#param-skip-push-event-for-pr-commits}

skip-push-event-for-pr-commits

boolean

default:"true"

Prevents duplicate PipelineRuns when a commit appears in both a push event and a pull request. If the pushed commit belongs to an open pull request, Pipelines-as-Code skips the push event.

{{< callout type="info" >}}
This does not apply to git tag push events, which always trigger pipeline runs.
{{< /callout >}}

### Auto-Configuration

## auto-configure-new-github-repo {#param-auto-configure-new-github-repo}

auto-configure-new-github-repo

boolean

default:"false"

Pipelines-as-Code automatically configures newly created GitHub repositories by creating a namespace and a Repository CR. Only GitHub Apps support this feature.

Verify that the `repository` event is subscribed in your GitHub App settings before you enable auto-configuration.

## auto-configure-repo-namespace-template {#param-auto-configure-repo-namespace-template}

auto-configure-repo-namespace-template

string

default:"{{repo\_name}}-pipelines"

The template Pipelines-as-Code uses to generate namespace names for auto-configured repositories. Available variables:

- `{{repo_owner}}` - The repository owner
- `{{repo_name}}` - The repository name

Example: `{{repo_owner}}-{{repo_name}}-ci` produces `owner-repo-ci` for `https://github.com/owner/repo`

## auto-configure-repo-repository-template {#param-auto-configure-repo-repository-template}

auto-configure-repo-repository-template

string

default:"{{repo\_name}}-repo-cr"

The template Pipelines-as-Code uses to generate Repository CR names for auto-configured repositories. Available variables:

- `{{repo_owner}}` - The repository owner
- `{{repo_name}}` - The repository name

### Security Settings

## remember-ok-to-test {#param-remember-ok-to-test}

remember-ok-to-test

boolean

default:"false"

When you enable this setting, Pipelines-as-Code automatically re-runs CI on pull request updates after the initial `/ok-to-test` approval, without requiring a new approval comment.

Enabling `remember-ok-to-test` creates security risks. An attacker could submit a harmless PR to gain trust, then inject malicious code in a later commit to exfiltrate secrets. Only enable if absolutely necessary.

## require-ok-to-test-sha {#param-require-ok-to-test-sha}

require-ok-to-test-sha

boolean

default:"false"

Requires `/ok-to-test` comments to include the specific commit SHA. This prevents race conditions where an attacker pushes malicious code after approval but before CI runs. Example: `/ok-to-test sha=abc123def456`

### Bitbucket Cloud Settings

## bitbucket-cloud-check-source-ip {#param-bitbucket-cloud-check-source-ip}

bitbucket-cloud-check-source-ip

boolean

default:"true"

Pipelines-as-Code verifies webhook requests from Bitbucket Cloud by checking them against Atlassian IP ranges. This check only applies to public Bitbucket (when `provider.url` is not set in your Repository CR spec).

Disabling `bitbucket-cloud-check-source-ip` is a security risk. Malicious users could send fake webhook payloads to trigger unauthorized PipelineRuns.

## bitbucket-cloud-additional-source-ip {#param-bitbucket-cloud-additional-source-ip}

bitbucket-cloud-additional-source-ip

string

default:""

Additional IPs or networks to allow for Bitbucket Cloud webhooks. You can specify individual IPs (`127.0.0.1`) or CIDR ranges (`127.0.0.0/16`). Separate multiple values with commas.

### Dashboard Integration

## tekton-dashboard-url {#param-tekton-dashboard-url}

tekton-dashboard-url

string

default:""

The URL of your Tekton Dashboard. When you set this, Pipelines-as-Code generates links to PipelineRun status and task logs pointing to your dashboard.

### Custom Console Configuration

## custom-console-name {#param-custom-console-name}

custom-console-name

string

default:""

The name of your custom console (e.g., "MyCorp Console").

## custom-console-url {#param-custom-console-url}

custom-console-url

string

default:""

The root URL of your custom console (e.g., "<https://mycorp.com>").

## custom-console-url-pr-details {#param-custom-console-url-pr-details}

custom-console-url-pr-details

string

default:""

A URL template for viewing PipelineRun details. You can use these template variables:

- `{{namespace}}` - Target namespace
- `{{pr}}` - PipelineRun name
- Any custom parameters from your Repository CR

Example: `https://mycorp.com/ns/{{namespace}}/pipelinerun/{{pr}}`

## custom-console-url-pr-tasklog {#param-custom-console-url-pr-tasklog}

custom-console-url-pr-tasklog

string

default:""

A URL template for viewing task logs. You can use these template variables:

- `{{namespace}}` - Target namespace
- `{{pr}}` - PipelineRun name
- `{{task}}` - Task name
- `{{pod}}` - Pod name
- `{{firstFailedStep}}` - First failed step name

Example: `https://mycorp.com/ns/{{namespace}}/pr/{{pr}}/logs/{{task}}#{{pod}}-{{firstFailedStep}}`

## Example ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: pipelines-as-code
  namespace: pipelines-as-code
data:
  application-name: "Pipelines as Code CI"
  secret-auto-create: "true"
  secret-github-app-token-scoped: "true"
  remote-tasks: "true"
  hub-url: "https://artifacthub.io"
  hub-catalog-type: "artifacthub"
  error-log-snippet: "true"
  error-log-snippet-number-of-lines: "3"
  error-detection-from-container-logs: "true"
  error-detection-max-number-of-lines: "50"
  enable-cancel-in-progress-on-pull-requests: "false"
  enable-cancel-in-progress-on-push: "false"
  bitbucket-cloud-check-source-ip: "true"
  auto-configure-new-github-repo: "false"
  remember-ok-to-test: "false"
  skip-push-event-for-pr-commits: "true"
```

## Applying Configuration Changes

To update the configuration:

```bash
kubectl edit configmap pipelines-as-code -n pipelines-as-code
```

Or apply changes from a file:

```bash
kubectl apply -f pipelines-as-code-config.yaml
```

Most configuration changes take effect immediately. Some settings may require a controller restart:

```bash
kubectl rollout restart deployment/pipelines-as-code-controller -n pipelines-as-code
```

## See Also

- [Global Repository Settings]({{< relref "global-repository-settings" >}}) - Configure default settings for all repositories
- [Logging Configuration]({{< relref "logging" >}}) - Configure log levels and debugging
- [Metrics]({{< relref "metrics" >}}) - Monitor Pipelines-as-Code with Prometheus
