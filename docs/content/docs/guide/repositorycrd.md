---
title: Repository CR
weight: 1
---
# Repository CR

The Repository CR serves the following purposes:

- Informing Pipelines-as-Code that an event from a specific URL needs to be handled.
- Specifying the namespace where the `PipelineRuns` will be executed.
- Referencing an API secret, username, or API URL if necessary for Git provider
  platforms that require it (e.g., when using webhooks instead of the GitHub
  application).
- Providing the last `PipelineRun`statuses for the repository (5 by default).
- Let you declare [custom parameters]({{< relref "/docs/guide/customparams" >}})
  within the `PipelineRun`that can be expanded based on certain filters.

The process involves creating a Repository CR inside the target namespace
my-pipeline-ci, using the tkn pac CLI or another method.

For example, this will create a Repo CR for the github repository
<https://github.com/linda/project>

```yaml
cat <<EOF|kubectl create -n my-pipeline-ci -f-
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: project-repository
spec:
  url: "https://github.com/linda/project"
EOF
```

With this configuration when an event from the `linda/project` repository
occurs, Pipelines-as-Code will know it needs to be handled and begin checking
out the contents of linda/project to match with the PipelineRun in the .tekton/
directory.

If the `PipelineRun` matches via its annotations the event, for example on a
specific branch and event like a `push` or `pull_request`. It will start the
`PipelineRun` where the `Repository` CR has been created. You can only start the
`PipelineRun` in the namespace where the Repository CR is located.

## Setting PipelineRun definition source

An additional layer of security can be added by using a PipelineRun annotation
to explicitly target a specific namespace. However, a Repository CRD must still
be created in that namespace for it to be matched.

This annotation helps prevent bad actors on a cluster from hijacking
PipelineRun execution to a namespace they don't have access to. It let the user
specify the ownership of a repo matching the access of a specific namespace on
a cluster

To use this feature, add the following annotation to the pipeline:

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

Pipelines-as-Code will then only match the repository in the mynamespace
namespace instead of trying to match it from all available repositories on the
cluster.

{{< hint info >}}
Pipelines-as-Code installs a Kubernetes Mutating Admission Webhook to ensure
that only one Repository CRD is created per URL on a cluster.

If you disable this webhook, multiple Repository CRDs can be created for the
same URL. However, only the oldest created Repository CRD will be matched,
unless you use the `target-namespace` annotation.
{{< /hint >}}

### PipelineRun definition provenance

By default on a `Push` or a `Pull Request`, Pipelines-as-Code will fetch the
PipelineRun definition from the branch of where the event has been triggered.

This behavior can be changed by setting the setting `pipelinerun_provenance`.
The setting currently accept two values:

- `source`: The default behavior, the PipelineRun definition will be fetched
  from the branch of where the event has been triggered.
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
Letting the user specify the provenance of the PipelineRun definition to default
branch is another layer of security. It ensures that only the one who has the
right to merge commit to the default branch can change the PipelineRun and have
access to the infrastrucutre.
{{< /hint >}}

## Concurrency

`concurrency_limit` allows you to define the maximum number of PipelineRuns running at any time for a Repository.

```yaml
spec:
  concurrency_limit: <number>
```

If there is multiple PipelineRuns matching the event, the PipelineRuns
that match the event will always be started in alphabetical order.

Example:

If you have three pipelineruns in a .tekton directory, and you create a pull
request with a `concurrency_limit` of 1 in the repository configuration, then all
of the pipelineruns will be executed in alphabetical order, one after the
other. At any given time, only one pipeline run will be in the running state,
while the rest will be queued.

## Scoping GitHub token to a list of private and public repositories within and outside namespaces

By default, the GitHub token that Pipelines-as-Code generates is scoped only to the repository where the payload comes from.
However, in some cases, the developer team might want the token to allow control over additional repositories.
For example, there might be a CI repository where the `.tekton/pr.yaml` file and source payload might be located, however the build process defined in `pr.yaml` might fetch tasks from a separate private CD repository.

You can extend the scope of the GitHub token in two ways:

- _Global configuration_: extend the GitHub token to a list of repositories in different namespaces and admin have access to set this configuration.

- _Repository level configuration_: extend the GitHub token to a list of repositories that exist in the same namespace as the original repository
and both admin, non-admin have access to set this configuration.

{{< hint info >}}
when using a GitHub webhook, the scoping of the token is what you set when creating your [fine grained personal access token](https://github.blog/2022-10-18-introducing-fine-grained-personal-access-tokens-for-github/#creating-personal-access-tokens).
{{</ hint >}}

Prerequisite

- In the `pipelines-as-code` configmap, set the `secret-github-app-token-scoped` key to `false`.
This setting enables the scoping of the GitHub token to private and public repositories listed under the Global and Repository level configuration.

### Scoping the GitHub token using Global configuration

You can use the global configuration to use as a list of repositories used from any Repository CR in any namespaces.

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

If any of the repositories does not exist in the namespace, the scoping of the GitHub token fails with an error message as in the following example:

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
  failed to scope github token as repo owner5/project5 does not exist in namespace test-repo
  ```
