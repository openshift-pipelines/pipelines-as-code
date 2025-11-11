---
title: Repository CR
weight: 1
---

# Repository CR

The Repository CR serves the following purposes:

- Informing Pipelines-as-Code that an event from a specific URL needs to be handled.
- Specifying the namespace where the `PipelineRuns` will be executed.
- Referencing an API secret, username, or API URL if necessary for Git provider
  platforms (e.g., when using webhooks instead of the GitHub
  application).
- Providing the last `PipelineRun` statuses for the repository (5 by default).
- Letting you declare [custom parameters]({{< relref "/docs/guide/customparams" >}})
  within the `PipelineRun` that can be expanded based on certain filters.

{{< hint danger >}}
The `pipelinerun_status` field in the `Repository` CR is scheduled for deprecation and will be removed in a future release. Please avoid relying on it.
{{< /hint >}}

To configure Pipelines-as-Code, a Repository CR must be created within the
user's namespace, for example `project-repository`, where their CI will run.

Note that you cannot create a Repository CR in the same namespace where
Pipelines-as-Code is deployed (for example
`openshift-pipelines` or `pipelines-as-code` namespace).

You can create the Repository CR using the [tkn pac]({{< relref
"/docs/guide/cli.md" >}}) CLI and its `tkn pac create repository` command or by
applying a YAML file with kubectl:

```bash
cat <<EOF | kubectl create -n project-repository -f-
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: project-repository
spec:
  url: "https://github.com/linda/project"
EOF
```

With this configuration, when an event from the `linda/project` repository
occurs, Pipelines-as-Code will know it needs to be handled and begin checking
out the contents of linda/project to match with the PipelineRun in the .tekton/
directory.

If the `PipelineRun` matches via its annotations the event, for example on a
specific branch and event like a `push` or `pull_request`, it will start the
`PipelineRun` where the `Repository` CR has been created. You can only start the
`PipelineRun` in the namespace where the Repository CR is located.

{{< hint info >}}
Pipelines-as-Code uses a Kubernetes Mutating Admission Webhook to enforce a
single Repository CRD per URL in the cluster and to ensure that URLs are valid
and non-empty.

Disabling this webhook is not supported and may pose a security risk in
clusters with untrusted users, as it could allow one user to hijack another's
private repository and gain unauthorized control over it.

If the webhook were disabled, multiple Repository CRDs could be created for the
same URL. In this case, only the first created CRD would be recognized unless
the user specifies the `target-namespace` annotation in their PipelineRun.
{{< /hint >}}

## Setting PipelineRun definition source

An additional layer of security can be added by using a PipelineRun annotation
to explicitly target a specific namespace. However, a Repository CRD must still
be created in that namespace for it to be matched.

This annotation helps prevent bad actors on a cluster from hijacking
PipelineRun execution to a namespace they don't have access to. It lets the user
specify the ownership of a repo matching the access of a specific namespace on
a cluster.

To use this feature, add the following annotation to the pipeline:

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

Pipelines-as-Code will then only match the repository in the mynamespace
namespace instead of trying to match it from all available repositories on the
cluster.

### PipelineRun definition provenance

By default, on a `Push` or a `Pull Request`, Pipelines-as-Code will fetch the
PipelineRun definition from the branch where the event has been triggered.

This behavior can be changed by setting the setting `pipelinerun_provenance`.
The setting currently accepts two values:

- `source`: The default behavior, the PipelineRun definition will be fetched
  from the branch where the event has been triggered.
- `default_branch`: The PipelineRun definition will be fetched from the default
  branch of the repository as configured on the git platform. For example
  `main`, `master`, or `trunk`.

Example:

This configuration specifies a repository named my-repo with a URL of
<https://github.com/my-org/my-repo>. It also sets the `pipelinerun_provenance`
setting to `default_branch`, which means that the PipelineRun definition will be
fetched from the default branch of the repository.

```yaml
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: my-repo
spec:
  url: "https://github.com/owner/repo"
  settings:
    pipelinerun_provenance: "default_branch"
```

{{< hint info >}}
Letting the user specify the provenance of the PipelineRun definition to the default
branch is another layer of security. It ensures that only the one who has the
right to merge commits to the default branch can change the PipelineRun and have
access to the infrastructure.
{{< /hint >}}

## Controlling Pull/Merge Request comment volume

For GitHub (Webhook) and GitLab integrations, you can control the types
of Pull/Merge request comments that Pipelines as Code emits using
the `spec.<provider>.comment_strategy` setting. This can
help reduce notification volume for repositories that use long-lasting
Pull/Merge requests with many PipelineRuns.

Acceptable values for `spec.<provider>.comment_strategy` are `""`
(default) and `"disable_all"`.

When you set the value of `comment_strategy` to `disable_all`, Pipelines
as Code will not add any comment on the Pull/Merge Request related to
PipelineRun status.

Note: The `disable_all` strategy applies only to comments about a
PipelineRun's status (e.g., "started," "succeeded"). Comments may still
appear if there are errors validating PipelineRuns in the `.tekton`
directory. (See [Running the PipelineRun docs](../running/#errors-when-parsing-pipelinerun-yaml) for details)

### GitLab

By default, Pipelines-as-Code attempts to update the commit status through the
GitLab API. It first tries the source project (fork), then falls back to the
target project (upstream repository). The source project update succeeds when
the configured token has access to the source repository and GitLab creates
pipeline entries for external CI systems like Pipelines-as-Code. The target
project fallback may fail if there's no CI pipeline running for that commit
in the target repository, since GitLab only creates pipeline entries for commits
that actually trigger CI in that specific project. If either status update
succeeds, no comment is posted on the Merge Request.

When a status update succeeds, you can see the status in the GitLab UI in the `Pipelines` tab, as
shown in the following example:

![Gitlab Pipelines from Pipelines-as-Code](/images/gitlab-pipelines-tab.png)

Comments are only posted when:

- Both commit status updates fail (typically due to insufficient token permissions)
- The event type and repository settings allow commenting
- The `comment_strategy` is not set to `disable_all`

```yaml
spec:
  settings:
    gitlab:
      comment_strategy: "disable_all"
```

### GitHub Webhook

```yaml
spec:
  settings:
    github:
      comment_strategy: "disable_all"
```

## Concurrency

`concurrency_limit` allows you to define the maximum number of PipelineRuns running at any time for a Repository.

```yaml
spec:
  concurrency_limit: <number>
```

When multiple PipelineRuns match the event, they will be started in alphabetical order by PipelineRun name.

Example:

If you have three PipelineRuns in a .tekton directory, and you create a pull
request with a `concurrency_limit` of 1 in the repository configuration, then all
of the PipelineRuns will be executed in alphabetical order, one after the
other. At any given time, only one PipelineRun will be in the running state,
while the rest will be queued.

### Kueue - Kubernetes-native Job Queueing

Pipelines-as-Code now accommodates [Kueue](https://kueue.sigs.k8s.io/) as an alternative, Kubernetes-native solution for queuing PipelineRun.
To get started, you can deploy the experimental integration provided by the [konflux-ci/tekton-kueue](https://github.com/konflux-ci/tekton-kueue) project. This allows you to schedule PipelineRuns through Kueue's queuing mechanism.

Note: The [konflux-ci/tekton-kueue](https://github.com/konflux-ci/tekton-kueue) project and the Pipelines-as-Code integration is only intended for testing.
It is only meant for experimentation and should not be used in production environments.

## Scoping GitHub token to a list of private and public repositories within and outside namespaces

By default, the GitHub token that Pipelines-as-Code generates is scoped only to the repository where the payload comes from.
However, in some cases, the developer team might want the token to allow control over additional repositories.
For example, there might be a CI repository where the `.tekton/pr.yaml` file and source payload might be located, however, the build process defined in `pr.yaml` might fetch tasks from a separate private CD repository.

You can extend the scope of the GitHub token in two ways:

- _Global configuration_: extend the GitHub token to a list of repositories in different namespaces and admin have access to set this configuration.

- _Repository level configuration_: extend the GitHub token to a list of repositories that exist in the same namespace as the original repository
and both admin and non-admin have access to set this configuration.

{{< hint info >}}
When using a GitHub webhook, the scoping of the token is what you set when creating your [fine-grained personal access token](https://github.blog/2022-10-18-introducing-fine-grained-personal-access-tokens-for-github/#creating-personal-access-tokens).
{{</ hint >}}

Prerequisite

- In the `pipelines-as-code` configmap, set the `secret-github-app-token-scoped` key to `false`.
This setting enables the scoping of the GitHub token to private and public repositories listed under the Global and Repository level configuration.

### Scoping the GitHub token using Global configuration

You can use the global configuration to use as a list of repositories used from any Repository CR in any namespace.

To set the global configuration, in the `pipelines-as-code` configmap, set the `secret-github-app-scope-extra-repos` key, as in the following example:

  ```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: pipelines-as-code
    namespace: pipelines-as-code
  data:
    secret-github-app-scope-extra-repos: "owner2/project2, owner3/project3"
  ```

### Scoping the GitHub token using Repository level configuration

You can use the `Repository` custom resource to scope the generated GitHub token to a list of repositories.
The repositories can be public or private, but must reside in the same namespace as the repository with which the `Repository` resource is associated.

Set the `github_app_token_scope_repos` spec configuration within the `Repository` custom resource, as in the following example:

  ```yaml
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: test
    namespace: test-repo
  spec:
    url: "https://github.com/linda/project"
    settings:
      github_app_token_scope_repos:
      - "owner/project"
      - "owner1/project1"
  ```

In this example, the `Repository` custom resource is associated with the `linda/project` repository in the `test-repo` namespace.
The scope of the generated GitHub token is extended to the `owner/project` and `owner1/project1` repositories, as well as the `linda/project` repository. These repositories must exist under the `test-repo` namespace.

**Note:**

If any of the repositories do not exist in the namespace, the scoping of the GitHub token fails with an error message as in the following example:

```console
failed to scope GitHub token as repo owner1/project1 does not exist in namespace test-repo
```

### Combining global and repository level configuration

- When you provide both a `secret-github-app-scope-extra-repos` key in the `pipelines-as-code` configmap and
a `github_app_token_scope_repos` spec configuration in the `Repository` custom resource, the token is scoped to all the repositories from both configurations, as in the following example:

  - `pipelines-as-code` configmap:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: pipelines-as-code
      namespace: pipelines-as-code
    data:
      secret-github-app-scope-extra-repos: "owner2/project2, owner3/project3"
    ```

  - `Repository` custom resource

    ```yaml
     apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
     kind: Repository
     metadata:
       name: test
       namespace: test-repo
     spec:
       url: "https://github.com/linda/project"
       settings:
         github_app_token_scope_repos:
         - "owner/project"
         - "owner1/project1"
    ```

    The GitHub token is scoped to the following repositories: `owner/project`, `owner1/project1`, `owner2/project2`, `owner3/project3`, `linda/project`.

- If you set only the global configuration in the `secret-github-app-scope-extra-repos` key in the `pipelines-as-code` configmap,
the GitHub token is scoped to all the listed repositories, as well as the original repository from which the payload files come.

- If you set only the `github_app_token_scope_repos` spec in the `Repository` custom resource,
the GitHub token is scoped to all the listed repositories, as well as the original repository from which the payload files come.
All the repositories must exist in the same namespace where the `Repository` custom resource is created.

- If you did not install the GitHub app for any repositories that you list in the global or repository level configuration,
creation of the GitHub token fails with the following error message:

    ```text
    failed to scope token to repositories in namespace test-repo with error : could not refresh installation id 36523992's token: received non 2xx response status \"422 Unprocessable Entity\" when fetching https://api.github.com/app/installations/36523992/access_tokens: Post \"https://api.github.com/repos/savitaashture/article/check-runs\
    ```

- If the scoping of the GitHub token to the repositories set in global or repository level configuration fails for any reason,
the CI process does not run. This includes cases where the same repository is listed in the global or repository level configuration,
and the scoping fails for the repository level configuration because the repository is not in the same namespace as the `Repository` custom resource.

  In the following example, the `owner5/project5` repository is listed in both the global configuration and in the repository level configuration:

  ```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: pipelines-as-code
    namespace: pipelines-as-code
  data:
    secret-github-app-scope-extra-repos: "owner5/project5"
  ```

  ```yaml
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: test
    namespace: test-repo
  spec:
    url: "https://github.com/linda/project"
    settings:
      github_app_token_scope_repos:
      - "owner5/project5"
  ```

  In this example, if the `owner5/project5` repository is not under the `test-repo` namespace, scoping of the GitHub token fails with the following error message:

  ```yaml
  failed to scope GitHub token as repo owner5/project5 does not exist in namespace test-repo
  ```
