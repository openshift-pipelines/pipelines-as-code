---
title: Resolver
weight: 2
---
# Pipelines as Code resolver

Pipelines as Code resolver ensures the PipelineRun you are running does not
conflicts with others.

If `Pipelines as Code` sees a PipelineRun with a reference to a `Task` or to a
`Pipeline` in any YAML file located in the `.tekton/` directory it will
automatically try to *resolves* it (see below) as a single PipelineRun with an
embedded `PipelineSpec` to a `PipelineRun`.

The resolver will then transform the Pipeline `Name` field to a `GenerateName`
based on the Pipeline name as well.

If you want to split your Pipeline and PipelineRun, you can store  the files in the
`.tekton/` directory or its subdirectories. `Pipeline` and `Task` can as well be
referenced remotely (see below on how the remote tasks are referenced).

The resolver will skip resolving if it sees these type of tasks:

* a reference to a [`ClusterTask`](https://github.com/tektoncd/pipeline/blob/main/docs/tasks.md#task-vs-clustertask)
* a `Task` or `Pipeline` [`Bundle`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#tekton-bundles)
* a reference to a Tekton [`Resolver`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#tekton-bundles)
* a [Custom Task](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#using-custom-tasks) with an apiVersion that doesn't have a `tekton.dev/` prefix.

It just uses them "as is" and will not try to do anything with it.

If Pipelines as Code cannot resolve the referenced tasks in the `Pipeline` or
`PipelineSpec`, the run will fail before applying the pipelinerun onto the
cluster.

You should be able to see the issue on your Git provider platform interface and
inside the events of the target namespace where the `Repository` CR  is
located.

If you need to test your `PipelineRun` locally before sending it in a PR, you
can use the `resolve` command from the `tkn-pac` CLI See  [CLI](./cli/#resolve)
command to learn on how to use it.

## Remote Task annotations

`Pipelines as Code` support fetching remote tasks or pipeline from a remote
location with annotations on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task or a Pipeline in
a PipelineRun or a PipelineSpec it will automatically inlines them.

An annotation to a remote task looks like this :

```yaml
pipelinesascode.tekton.dev/task: "git-clone"
```

or multiple tasks with an array :

```yaml
pipelinesascode.tekton.dev/task: "[git-clone, pylint]"
```

### [Tekton Hub](https://hub.tekton.dev)

```yaml
pipelinesascode.tekton.dev/task: "git-clone"
```

The syntax above installs the
[git-clone](https://github.com/tektoncd/catalog/tree/main/task/git-clone) task
from the [tekton hub](https://hub.tekton.dev) repository querying for the latest
version with the tekton hub API.

You can have multiple tasks in there if you separate them by a comma `,` around
an array syntax with bracket:

```yaml
pipelinesascode.tekton.dev/task: "[git-clone, golang-test, tkn]"
```

You can have multiple lines if you add a `-NUMBER` suffix to the annotation, for
example :

```yaml
  pipelinesascode.tekton.dev/task: "git-clone"
  pipelinesascode.tekton.dev/task-1: "golang-test"
  pipelinesascode.tekton.dev/task-2: "tkn"
```

By default, `Pipelines as Code` will interpret the string as the `latest` task to
grab
from [tekton hub](https://hub.tekton.dev).

If you want to have a specific version of the task, you can add a colon `:` to
the string and a version number, like in
this example :

```yaml
pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # this will install git-clone 0.1 from tekton.hub
```

### Remote HTTP URL

If you have a string starting with `http://` or `https://`, `Pipelines as Code`
will fetch the task directly from that remote URL :

```yaml
  pipelinesascode.tekton.dev/task: "[https://remote.url/task.yaml]"
```

### Remote HTTP URL from a private Github repository

If you are using `GitHub` and If the remote task URL uses the same host as where
the repo CRD is, PAC will use the  GitHub token and fetch the URL using the
Github API.

For example if you have a repo URL looking like this :

<https://github.com/organization/repository>

and the remote HTTP URLs is a referenced GitHub "blob" URL:

<https://github.com/organization/repository/blob/mainbranch/path/file>

or a Github rawURL (rawurl reference is only working on public GitHub):

<https://raw.githubusercontent.com/organization/repository/mainbranch/path/file>

It will be able to fetch the files from that private repository with the GitHub app token.

This allows you to reference a task or a pipeline from a private repository easily.

Github app token are scoped to the owner or organization where the repository is located.
If you are using the GitHub webhook method you are able to fetch any private or
public repositories on any organization where the personal token is allowed.

There is settings you can set in the pac `Configmap` to control that behavior, see the
`secret-github-app-token-scoped` and `secret-github-app-scope-extra-repos` settings in the
[settings documentation](/docs/install/settings).

### Tasks or Pipelines inside the repository

Additionally, you can as well have a reference to a task or pipeline from a YAML file inside
your repo if you specify the relative path to it, for example :

```yaml
pipelinesascode.tekton.dev/task: "[share/tasks/git-clone.yaml]"
```

This will grab the file `share/tasks/git-clone.yaml` from the current
repository on the `SHA` where the event come from (i.e: the current pull
request or the current branch push).

If there is any error fetching those resources, `Pipelines as Code` will error
out and not process the pipeline.

If the object fetched cannot be parsed as a Tekton `Task` it will error out.

## Remote Pipeline annotations

Remote Pipeline can be referenced by annotation, this allows you to share your Pipeline definition across.

Only one Pipeline is allowed in annotation.

An annotation to a remote pipeline looks like this :

```yaml
pipelinesascode.tekton.dev/pipeline: "https://git.provider/raw/pipeline.yaml
```

It supports remote URL and files inside the same Git repository.

{{< hint info >}}
[Tekton Hub](https://hub.tekton.dev) doesn't currently have support for `Pipeline`.
{{< /hint >}}
