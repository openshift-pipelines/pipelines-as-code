---
title: Authoring PipelineRun
weight: 3
---
# Authoring PipelineRuns in `.tekton/` directory

* Pipelines as Code will always try to be as close to the tekton template as
  possible. Usually you will write your template and save them with a `.yaml`
  extension and Pipelines as Code will run them.

* The `.tekton` directory must be at the top level of the repo.
  You can reference YAML files in other repos using remote URLs
  (see [Remote HTTP URLs](./resolver.md#remote-http-url) for more information),
  but PipelineRuns will only be triggered by events in the repository containing
  the `.tekton` directory.

* Using its [resolver](../resolver/) Pipelines as Code will try to bundle the
  PipelineRun with all its Task as a single PipelineRun with no external
  dependencies.

* Inside your pipeline you need to be able to check out the commit as
  received from the webhook by checking it out the repository from that ref. Most of the time
  you want to reuse the
  [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from the [tektoncd/catalog](https://github.com/tektoncd/catalog).

  To be able to specify the parameters of your commit and URL, Pipelines as Code
  allows you to have those "dynamic" variables expanded. Those variables look
  like this `{{ var }}` and those are the one you can use:

  * `{{repo_owner}}`: The repository owner.
  * `{{event_type}}`: The event type (eg: `pull_request` or `push`)
  * `{{repo_name}}`: The repository name.
  * `{{repo_url}}`: The repository full URL.
  * `{{target_namespace}}`: The target namespace where the Repository has matched and the PipelineRun will be created.
  * `{{revision}}`: The commit full sha revision.
  * `{{sender}}`: The sender username (or accountid on some providers) of the commit.
  * `{{source_branch}}`: The branch name where the event come from.
  * `{{target_branch}}`: The branch name on which the event targets (same as `source_branch` for push events).
  * `{{pull_request_number}}`: The pull or merge request number, only defined when we are in a `pull_request` event type.
  * `{{git_auth_secret}}`: The secret name auto generated with provider token to check out private repos.

* For Pipelines as Code to process your `PipelineRun`, you must have either an
  embedded `PipelineSpec` or a separate `Pipeline` object that references a YAML
  file in the `.tekton` directory. The Pipeline object can include `TaskSpecs`,
  which may be defined separately as Tasks in another YAML file in the same
  directory. It's important to give each `PipelineRun` a unique name to avoid
  conflicts. **Duplicate names are not permitted**.

## Matching an event to a PipelineRun

Each `PipelineRun` can match different Git provider events through some special
annotations on the `PipelineRun`. For example when you have these metadatas in
your `PipelineRun`:

```yaml
 metadata:
    name: pipeline-pr-main
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines as Code` will match the pipelinerun `pipeline-pr-main` if the Git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branch can be specified separated by comma, i.e:

```yaml
[main, release-nightly]
```

You can match on `pull_request` events as above, and you can as well match
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

If there are multiple pipelinerun matching an event, it will run all of them in
parallel and posting the results to the provider as soon the PipelineRun
finishes.

## Advanced event matching

If you need to do some advanced matching, `Pipelines as Code` supports CEL
filtering.

If you have the ``pipelinesascode.tekton.dev/on-cel-expression`` annotation in
your PipelineRun, the CEL expression will be used and the `on-target-branch` or
`on-target-branch` annotations will then be skipped.

This example will match a `pull_request` event targeting the branch `main`
coming from a branch called `wip`:

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && target_branch == "main" && source_branch == "wip"
```

Another example, if you want to have a PipelineRun running only if a path has
changed you can use the `.pathChanged` suffix function with a [glob
pattern](https://github.com/ganbarodigital/go_glob#what-does-a-glob-pattern-look-like). Here
is a concrete example matching every markdown files (as files who has the `.md`
suffix) in the `docs` directory :

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && "docs/*.md".pathChanged()
```

This example will match all pull request starting with the title `[DOWNSTREAM]`:

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request && event_title.startsWith("[DOWNSTREAM]")
```

The fields available are :

* `event`: `push` or `pull_request`
* `target_branch`: The branch we are targeting.
* `source_branch`: The branch where this pull_request come from. (on `push` this
  is the same as `target_branch`).
* `target_url`: The url of the repository we are targeting.
* `source_url`: The url of the repository where this pull_request come from. (on `push` this
  is the same as `target_url`).
* `event_title`: Match the title of the event. When doing a push this will match
  the commit title and when matching on PR it will match the Pull or Merge
  Request title. (only `GitHub`, `Gitlab` and `BitbucketCloud` providers are supported)
* `.pathChanged`: a suffix function to a string which can be a glob of a path to
  check if changed (only `GitHub` and `Gitlab` provider is supported)

Compared to the simple "on-target" annotation matching, the CEL expression
allows you to complex filtering and most importantly express negation.

For example if I want to have a `PipelineRun` targeting a `pull_request` but
not the `experimental` branch I would have :

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      event == "pull_request" && target_branch != experimental"
```

You can find more information about the CEL language spec here :

<https://github.com/google/cel-spec/blob/master/doc/langdef.md>

## Using the temporary GitHub APP Token for GitHub API operations

You can use the temporary installation token that is generated by Pipelines as
Code from the GitHub App to access the GitHub API.

The token value is stored into the temporary git-auth secret as generated for [private
repositories](../privaterepo/) in the key `git-provider-token`.

As an example if you want to add a comment to your pull request, you can use the
[github-add-comment](https://hub.tekton.dev/tekton/task/github-add-comment)
task from the [Tekton Hub](https://hub.tekton.dev)
using a [pipelines as code annotation](../resolver/#remote-http-url):

```yaml
  pipelinesascode.tekton.dev/task: "github-add-comment"
```

you can then add the task to your [tasks section](https://tekton.dev/docs/pipelines/pipelines/#adding-tasks-to-the-pipeline) (or [finally](https://tekton.dev/docs/pipelines/pipelines/#adding-finally-to-the-pipeline) tasks) of your PipelineRun :

```yaml
[...]
tasks:
  - name:
      taskRef:
        name: github-add-comment
      params:
        - name: REQUEST_URL
          value: "{{ repo_url }}/pull/{{ pull_request_number }}"
        - name: COMMENT_OR_FILE
          value: "Pipelines as Code IS GREAT!"
        - name: GITHUB_TOKEN_SECRET_NAME
          value: "{{ git_auth_secret }}"
        - name: GITHUB_TOKEN_SECRET_KEY
          value: "git-provider-token"
```

Since we are using the dynamic variables we are able to reuse this on any
PullRequest from any repositories.

and for completeness, here is another example how to set the GITHUB_TOKEN
environment variable on a task step:

```yaml
env:
  - name: GITHUB_TOKEN
    valueFrom:
      secretKeyRef:
        name: "{{ git_auth_secret }}"
        key: "git-provider-token"

```

{{< hint info >}}

* On GitHub apps the generated installation token [will be available for 8 hours](https://docs.github.com/en/developers/apps/building-github-apps/refreshing-user-to-server-access-tokens)
* On GitHub apps the token is scoped to the repository the event (payload) come
  from unless [configured](/docs/install/settings#pipelines-as-code-configuration-settings) it differently on cluster.

{{< /hint >}}

## Example

`Pipelines as code` test itself, you can see the examples in its
[.tekton](https://github.com/openshift-pipelines/pipelines-as-code/tree/main/.tekton) repository.
