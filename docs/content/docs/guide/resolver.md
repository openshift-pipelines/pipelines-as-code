---
title: Resolver
weight: 2
---
# Pipelines as Code resolver

If `Pipelines as Code` sees a PipelineRun with a reference to a `Task` or a
`Pipeline`, `Pipelines as Code` will try to *resolves* it (see below) as a
single PipelineRun with an embedded `PipelineSpec` to a `PipelineRun`.

It will transform the Pipeline `Name` field to a `generateName` based on the
Pipeline name as well.

This allows you to have multiple runs in the same namespace from the same
PipelineRun with no risk of conflicts.

Everything that runs your pipelinerun and its references need to be inside the
`.tekton/` directory and subdirectories as referenced with a remote task (see
below on how the remote tasks are referenced).

If you have a taskRef to a task located in any directory or subdirectories of the
`.tekton/` directory it will be automatically embedded even if it's not in the
annotations.

The resolver will skip resolving if he sees these type of tasks:

* a reference to a [`ClusterTask`](https://github.com/tektoncd/pipeline/blob/main/docs/tasks.md#task-vs-clustertask)
* a `Task` or `Pipeline` [`Bundle`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#tekton-bundles)
* or a [Custom Task](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#using-custom-tasks) with an apiVersion that doesn't have a `tekton.dev/` prefix.

It just use them "as is" and will not try to do anything with it.

If pipelines as code cannot resolve the referenced tasks in the `Pipeline` or
`PipelineSpec`, the run will fail before applying the pipelinerun onto the
cluster. You should be able to the issue on your Git provider platform or
through the log of the controller.

If you need to test your `PipelineRun` locally before sending it in a PR, you
can use the `resolve` command from the `tkn-pac` CLI See the `--help` of the
command to learn about how to use it.

## Remote Task support

`Pipelines as Code` support fetching remote tasks from remote location through
annotations on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task through its name in
a Pipeline or a PipelineSpec it will automatically inlines it.

An annotation to a remote task looks like this :

```yaml
pipelinesascode.tekton.dev/task: "git-clone"
```

or multiple tasks with an array :

```yaml
pipelinesascode.tekton.dev/task: ["git-clone", "pylint"]
```

The syntax above installs the
[git-clone](https://github.com/tektoncd/catalog/tree/main/task/git-clone) task
from the [tekton hub](https://hub.tekton.dev) repository querying for the latest
one with the tekton hub API.

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
pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # will install git-clone 0.1 from tekton.hub
```

If you have a string starting with http:// or https://, `Pipelines as Code`
will fetch the task directly from that remote URL :

```yaml
  pipelinesascode.tekton.dev/task: "[https://raw.githubusercontent.com/tektoncd/catalog/main/task/git-clone/0.3/git-clone.yaml]"
```

Additionally, you can as well have a reference to a task from a YAML file inside your repo if you specify the relative path to it, for example :

```yaml
pipelinesascode.tekton.dev/task: "[share/tasks/git-clone.yaml]"
```

This will grab the file `share/tasks/git-clone.yaml` from the current
repository on the `SHA` where the event come from (i.e: the current pull
request or the current branch push).

If there is any error fetching those resources, `Pipelines as Code` will error
out and not process the pipeline.

If the object fetched cannot be parsed as a Tekton `Task` it will error out.
