---
title: Authoring PipelineRun
weight: 3
---
# Authoring PipelineRuns in `.tekton/` directory

- Pipelines as Code will always try to be as close to the tekton template as
  possible. Usually you will write your template and save them with a ".yaml"
  extension and Pipelines as Code will run them.

- Using its [resolver](./resolver) Pipelines as Code will try to bundle the
  PipelineRun with all its Task as a single PipelineRun with no external
  dependences.

- Inside your pipeline you need to be able to check out the commit as
  received from the webhook by checking it out the repository from that ref. You
  would usually use the
  [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from catalog.

  To be able to specify the parameters of your commit and url, Pipelines as Code
  allows you to have those "dynamic" variables expanded. Those variables look
  like this `{{ var }}`and those are the one you can use:

  - `{{repo_owner}}`: The repository owner.
  - `{{repo_name}}`: The repository name.
  - `{{repo_url}}`: The repository full URL.
  - `{{revision}}`: The commit full sha revision.
  - `{{sender}}`: The sender username (or account id on some providers) of the commit.
  - `{{source_branch}}`: The branch name where the event come from.
  - `{{target_branch}}`: The branch name on which the event targets (same as `source_branch` for push events).

- You need at least one `PipelineRun` with a `PipelineSpec` or a separated
  `Pipeline` object. You can have embedded `TaskSpec` inside
  `Pipeline` or you can have them defined separately as `Task`.

## Matching an event to a PipelineRun

Each `PipelineRun` can match different git provider events via some special
annotations on the `PipelineRun`. For example when you have these metadatas in
your `PipelineRun`:

```yaml
 metadata:
    name: pipeline-pr-main
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines as Code` will match the pipelinerun `pipeline-pr-main` if the git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branch can be specified separated by comma, i.e:

```yaml
[main, release-nightly]
```

You can match on `pull_request` events as above and you can as well match
pipelineRuns on `push` events to a repository

For example this will match the pipeline when there is a push to a commit in the
`main` branch :

```yaml
 metadata:
  name: pipeline-push-on-main
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/heads/main]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

You can specify the full refs like `refs/heads/main` or the shortref like
`main`. You can as well specify globs, for example `refs/heads/*` will match any
target branch or `refs/tags/1.*` will match all the tags starting from `1.`.

A full example for a push of a tag :

```yaml
 metadata:
 name: pipeline-push-on-1.0-tags
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/tags/1.0]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

This will match the pipeline `pipeline-push-on-1.0-tags` when you push the 1.0
tags into your repository.

Matching annotations are currently mandated or `Pipelines as Code` will not
match your `PipelineRun`.

If there is multiple pipeline matching an event, it will match the first one. We
are currently not supporting multiple PipelineRuns on a single event but this
may be something we can consider to implement in the future.

## Advanced event matching

If you need to do some advanced matching, `Pipelines as Code` supports CEL
filtering.

If you have the ``pipelinesascode.tekton.dev/on-cel-expression`` annotation in
your PipelineRun, the CEL expression will be used and the `on-target-branch` or
`on-target-branch` annotations will then be skipped.

For example :

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && target_branch == "main" && source_branch == "wip"
```

will match a `pull_request` event targetting the branch `main` coming from a branch called `wip`.

The fields available are :

* `event`: `push` or `pull_request`
* `target_branch`: The branch we are targetting.
* `source_branch`: The branch where this pull_request come from. (on `push` this is the same as `target_branch`).

Compared to the simple "on-target" annotation matching, the CEL expression
allows you to complex filtering and most importantly express negation.

For example if I want to have a `PipelineRun` targeting a `pull_request` but
not the `experimental` branch I would have :

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && target_branch != experimental"
```

You can find more information about the CEL language spec here :

https://github.com/google/cel-spec/blob/master/doc/langdef.md

## Example

`Pipelines as code` test itself, you can see the examples in its
[.tekton](./../.tekton) repository.


