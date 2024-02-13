---
title: Resolver
weight: 2
---
# Pipelines-as-Code resolver

The Pipelines-as-Code resolver ensures that the PipelineRun you are running
doesn't conflict with others.

Pipelines-as-Code parses any files ending with a `.yaml` or `.yml` suffix in
the `.tekton` directory and subdirectory at the root of your repository. It
will automatically attempt to detect any [Tekton](https://tekton.dev) resources
like `Pipeline`, `PipelineRun`, `Task`.

When detecting a [PipelineRun](https://tekton.dev/docs/pipelines/pipelineruns/) it will try to *resolve*
it as a single PipelineRun with an embedded PipelineSpec of the referenced
[Task](https://tekton.dev/docs/pipelines/tasks/) or
[Pipeline](https://tekton.dev/docs/pipelines/pipelines/). This embedding
ensures that all dependencies required for execution are contained within a
single PipelineRun at the time of execution on the cluster.

{{< hint info >}}
The `Pipelines-as-Code` resolver is a different concept than the [Tekton resolver](https://tekton.dev/docs/pipelines/resolution-getting-started/), both are compatible and you can have the Tekton resolver within Pipelines-as-Code PipelineRun.
{{< /hint >}}

In any case of errors in any YAML files, parsing will halt, and errors will be
reported on both the Git provider interface and the event's Namespace stream.

The resolver will then transform the Pipeline `Name` field to a `GenerateName`
based on the Pipeline name to ensure each PipelineRun is unique.

If you want to split your Pipeline and PipelineRun, you can store  the files in the
`.tekton/` directory or its subdirectories. `Pipeline` and `Task` can as well be
referenced remotely (see below on how the remote tasks are referenced).

The resolver will skip resolving if it sees these type of tasks:

* a reference to a [`ClusterTask`](https://github.com/tektoncd/pipeline/blob/main/docs/tasks.md#task-vs-clustertask)
* a `Task` or `Pipeline` [`Bundle`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#tekton-bundles)
* a reference to a Tekton [`Resolver`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#tekton-bundles)
* a [Custom Task](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#using-custom-tasks) with an apiVersion that doesn't have a `tekton.dev/` prefix.

It just uses them "as is" and will not try to do anything with it.

If Pipelines-as-Code cannot resolve the referenced tasks in the `Pipeline` or
`PipelineSpec`, the run will fail before applying the pipelinerun onto the
cluster.

You should be able to see the issue on your Git provider platform interface and
inside the events of the target namespace where the `Repository` CR  is
located.

If you need to test your `PipelineRun` locally before sending it in a PR, you
can use the `resolve` command from the `tkn-pac` CLI See  [CLI](./cli/#resolve)
command to learn on how to use it.

## Remote Task annotations

`Pipelines-as-Code` support fetching remote tasks or pipeline from a remote
location with annotations on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task or a Pipeline in
a PipelineRun or a PipelineSpec it will automatically inline them.

If multiple annotations reference the same task name the resolver will pick the
first one fetched from the annotations.

An annotation to a remote task looks like this :

```yaml
pipelinesascode.tekton.dev/task: "git-clone"
```

or multiple tasks with an array :

```yaml
pipelinesascode.tekton.dev/task: "[git-clone, pylint]"
```

### [Tekton Hub](https://hub.tekton.dev) Support for Tasks

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

By default, `Pipelines-as-Code` will interpret the string as the `latest` task to
grab
from [tekton hub](https://hub.tekton.dev).

If you want to have a specific version of the task, you can add a colon `:` to
the string and a version number, like in
this example :

```yaml
pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # this will install git-clone 0.1 from tekton.hub
```

#### Custom [Tekton Hub](https://github.com/tektoncd/hub/) Support for Tasks

Additionally if the cluster administrator has [set-up](/docs/install/settings#tekton-hub-support) a custom Tekton Hub you
are able to reference it from your template like this example:

```yaml
pipelinesascode.tekton.dev/task: "[anothercatalog://curl]" # this will install curl from the custom catalog configured by the cluster administrator as anothercatalog
```

There is no fallback to the default Tekton Hub if the custom Tekton Hub does not
have the task referenced it will fail.

There is no support for custom hub from the CLI on the `tkn pac resolver` command.

### Remote HTTP URL

If you have a string starting with `http://` or `https://`, `Pipelines-as-Code`
will fetch the task directly from that remote URL :

```yaml
  pipelinesascode.tekton.dev/task: "[https://remote.url/task.yaml]"
```

### Remote HTTP URL from a private GitHub repository

If you are using `GitHub` and If the remote task URL uses the same host as where
the repository CRD is, Pipelines-as-Code will use the  GitHub token and fetch the URL using the
GitHub API.

For example if you have a repository URL looking like this :

<https://github.com/organization/repository>

and the remote HTTP URLs is a referenced GitHub "blob" URL:

<https://github.com/organization/repository/blob/mainbranch/path/file>

if the remote HTTP URL has a slash (/) in the branch name you will need to HTML
encode with the `%2F` character, example:

<https://github.com/organization/repository/blob/feature%2Fmainbranch/path/file>

It will be use the GitHub API with the generated token to fetch that file.
This allows you to reference a task or a pipeline from a private repository easily.

GitHub app token are scoped to the owner or organization where the repository is located.
If you are using the GitHub webhook method you are able to fetch any private or
public repositories on any organization where the personal token is allowed.

There is settings you can set in the Pipelines-as-Code `Configmap` to control that behaviour, see the
`secret-github-app-token-scoped` and `secret-github-app-scope-extra-repos` settings in the
[settings documentation](/docs/install/settings).

### Tasks or Pipelines inside the repository

Additionally, you can as well have a reference to a task or pipeline from a YAML file inside
your repository if you specify the relative path to it, for example :

```yaml
pipelinesascode.tekton.dev/task: "[share/tasks/git-clone.yaml]"
```

This will grab the file `share/tasks/git-clone.yaml` from the current
repository on the `SHA` where the event come from (i.e: the current pull
request or the current branch push).

If there is any error fetching those resources, `Pipelines-as-Code` will error
out and not process the pipeline.

If the object fetched cannot be parsed as a Tekton `Task` it will error out.

## Remote Pipeline annotations

Remote Pipeline can be referenced by annotation, allowing you to share a Pipeline across multiple repositories.

Only one pipeline annotation(pipelinesascode.tekton.dev/pipeline) for remote pipeline is allowed on the `PipelineRun`. Also, the
value of the annotation should have one pipeline. Annotations like `pipelinesascode.tekton.dev/pipeline-1` are not supported.

An annotation to a remote pipeline looks like this, using a remote URL:

```yaml
pipelinesascode.tekton.dev/pipeline: "https://git.provider/raw/pipeline.yaml
```

or from a relative path inside the repository:

```yaml
pipelinesascode.tekton.dev/pipeline: "./tasks/pipeline.yaml
```

### [Tekton Hub](https://hub.tekton.dev) Support for Pipelines

```yaml
pipelinesascode.tekton.dev/pipeline: "[buildpacks]"
```

The syntax above installs the
[buildpacks](https://github.com/tektoncd/catalog/tree/main/pipeline/buildpacks) pipeline
from the [tekton hub](https://hub.tekton.dev) repository querying for the latest
version with the tekton hub API.

If you want to have a specific version of the pipeline, you can add a colon `:` to
the string and a version number, like in this example :

```yaml
pipelinesascode.tekton.dev/pipeline: "[buildpacks:0.1]" # this will install buildpacks 0.1 from tekton hub
```

#### Custom [Tekton Hub](https://github.com/tektoncd/hub/) Support for Pipelines

Additionally if the cluster administrator has [set-up](/docs/install/settings#tekton-hub-support) a custom Tekton Hub you
are able to reference it from your template like this example:

```yaml
pipelinesascode.tekton.dev/pipeline: "[anothercatalog://buildpacks:0.1]" # this will install buildpacks from the custom catalog configured by the cluster administrator as anothercatalog
```

### Overriding tasks from a remote pipeline on a PipelineRun

{{< tech_preview "Tasks from a remote Pipeline override" >}}

Remote task annotations on the remote pipeline are supported. No other
annotations like `on-target-branch`, `on-event` or `on-cel-expression` are
supported.

If a user wants to override one of the tasks from the remote pipeline, they can do
so by adding a task in the annotations that has the same name In their `PipelineRun` annotations.

For example if the user PipelineRun contains those annotations:

```yaml
kind: PipelineRun
metadata:
  annotations:
    pipelinesascode.tekton.dev/pipeline: "https://git.provider/raw/pipeline.yaml
    pipelinesascode.tekton.dev/task: "./my-git-clone-task.yaml
```

And the Pipeline referenced by the `pipelinesascode.tekton.dev/pipeline` annotation
in "<https://git.provider/raw/pipeline.yaml>"  contains those annotations:

```yaml
kind: Pipeline
metadata:
  annotations:
    pipelinesascode.tekton.dev/task: "git-clone"
```

In this case if the `my-git-clone-task.yaml` file in the root directory is a
task named `git-clone` it will be used instead of the `git-clone` on the remote
pipeline that is coming from the Tekon Hub.

{{< hint info >}}
Task overriding is only supported for tasks that are referenced by a `taskRef`
to a `Name`, no override is done on `Tasks` embedded with a `taskSpec`. See
[Tekton documentation](https://tekton.dev/docs/pipelines/pipelines/#adding-tasks-to-the-pipeline) for the differences between `taskRef` and `taskSpec`:
{{< /hint >}}

### Tasks or Pipelines Precedence

From where tasks or pipelines of the same name takes precedence?

For the remote Tasks, when you have a `taskRef` on a task name, Pipelines-as-Code will try to find the task in this order:

1. A task matched from the PipelineRun annotations
2. A task matched from the remote Pipeline annotations
3. A task matched fetched from the Tekton directory
   (the tasks from the `.tekton` directory and its sub-directories are automatically included)

For the remote Pipeline referenced on a `pipelineRef`, Pipelines-as-Code will try to match a
pipeline in this order:

1. The Pipeline from the PipelineRun annotations
2. The Pipeline from the Tekton directory (pipelines are automatically fetched from
  the `.tekton` directory and its sub-directories)
