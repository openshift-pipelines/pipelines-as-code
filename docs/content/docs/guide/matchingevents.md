---
title: Matching a PipelineRun
weight: 3
---

# Matching a PipelineRun to a Git provider Event

A `PipelineRun` can be matched to a Git provider event by using specific
annotations in the `PipelineRun` metadata.

For example when you have these metadata in your `PipelineRun`:

```yaml
metadata:
  name: pipeline-pr-main
annotations:
  pipelinesascode.tekton.dev/on-target-branch: "[main]"
  pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines-as-Code` will match the pipelinerun `pipeline-pr-main` if the Git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branches can be specified, separated by commas, e.g.:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release-nightly]
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
`main`. You can also specify globs, for example `refs/heads/*` will match any
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

Matching annotations are currently required; otherwise `Pipelines-as-Code` will not
match your `PipelineRun`.

If there are multiple pipelinerun matching an event, it will run all of them in
parallel and posting the results to the provider as soon the PipelineRun
finishes.

{{< hint info >}}
Payload matching only occurs for events that `Pipelines-as-Code` supports, such as
when a `Pull Request` is opened, updated, or when a branch receives a `Push`.
{{< /hint >}}

## Matching a PipelineRun to Specific Path Changes

{{< tech_preview "Matching a PipelineRun to specific path changes via annotation" >}}

To trigger a `PipelineRun` based on specific path changes in an event, use the
annotation `pipelinesascode.tekton.dev/on-path-change`.

Multiple paths can be specified, separated by commas. The first glob matching
the files changes in the PR will trigger the `PipelineRun`. If you want to match
a file or path that has a comma you can html escape it with the `&#44;` html
entity.

You still need to specify the event type and target branch. If you have a [CEL
expression](#matching-pipelinerun-by-path-change) the `on-path-change`
annotation will be ignored

Example:

```yaml
metadata:
  name: pipeline-docs-and-manual
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[docs/**.md, manual/**.rst]"
```

This configuration will match and trigger the `PipelineRun` named
`pipeline-docs-and-manual` when a `pull_request` event targets the `main` branch
and includes changes to files with a `.md` suffix in the `docs` directory (and
its subdirectories) or files with a `.rst` suffix in the `manual` directory.

{{< hint info >}}
The patterns used are [glob](https://en.wikipedia.org/wiki/Glob_(programming))
patterns, not regexp. Here are some
[examples](https://github.com/gobwas/glob?tab=readme-ov-file#example) from the
library used for matching.

The `tkn pac` CLI provides a handy [globbing command]({{< relref "/docs/guide/cli.md#test-globbing-pattern" >}})
to test the glob pattern matching:

```bash
tkn pac info globbing "[PATTERN]"
```

will match the files with `[PATTERN]` in the current directory.

{{< /hint >}}

### Matching a PipelineRun by Ignoring Specific Path Changes

{{< tech_preview "Matching a PipelineRun to ignore specific path changes via annotation" >}}

Following the same principle as the `on-path-change` annotation, you can use the
reverse annotation `pipelinesascode.tekton.dev/on-path-change-ignore` to trigger
a `PipelineRun` when the specified paths have not changed.

You still need to specify the event type and target branch. If you have a [CEL
expression](#matching-pipelinerun-by-path-change) the `on-path-change-ignore`
annotation will be ignored

This PipelineRun will run when there are changes outside the docs
folder:

```yaml
metadata:
  name: pipeline-not-on-docs-change
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[docs/***]"
```

Furthermore, you can combine `on-path-change` and `on-path-change-ignore`
annotations:

```yaml
metadata:
  name: pipeline-docs-not-generated
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[docs/***]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[docs/generated/***]"
```

This configuration triggers the `PipelineRun` when there are changes in the
`docs` directory but not in the `docs/generated` directory.

The `on-path-change-ignore` annotation will always take precedence over the
`on-path-change` annotation, It means if you have these annotations:

```yaml
metadata:
  name: pipelinerun-go-only-no-markdown-or-yaml
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[***.go]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[***.md, ***.yaml]"
```

and you have a `Pull Request` changing the files `.tekton/pipelinerun.yaml`,
`README.md`, and `main.go` the `PipelineRun` will not be triggered since the
`on-path-change-ignore` annotation will ignore the `***.md` and `***.yaml`
files.

## Matching a PipelineRun on a regexp in a comment

{{< tech_preview "Matching PipelineRun on regexp in comments" >}}

You can match a PipelineRun on a comment on a Pull Request with the annotation
`pipelinesascode.tekton.dev/on-comment`.

The comment is a regexp and if a newly created comment has this regexp it will
automatically match the PipelineRun and starts it.

For example:

```yaml
---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: "merge-pr"
  annotations:
    pipelinesascode.tekton.dev/on-comment: "^/merge-pr"
```

This will match the merge-pr `PipelineRun` when a comment on a pull request
starts with `/merge-pr`.

When the PipelineRun that has been triggered with the `on-comment` annotation
gets started the template variable `{{ trigger_comment }}` get set. See the
documentation [here]({{< relref "/docs/guide/gitops_commands.md#accessing-the-comment-triggering-the-pipelinerun" >}})

Note that the `on-comment` annotation will respect the `pull_request` [Policy]({{< relref "/docs/guide/policy" >}}) rule,
so only users into the `pull_request` policy will be able to trigger the
PipelineRun.

> *NOTE*: The `on-comment` annotation is only supported on GitHub, Gitea and GitLab providers

## Advanced event matching using CEL

If you need to do some advanced matching, `Pipelines-as-Code` supports CEL
expression to do advanced filtering on the specific event you need to be matched.

If you have the `pipelinesascode.tekton.dev/on-cel-expression` annotation in
your PipelineRun, the CEL expression will be used and the `on-target-branch` or
`on-event` annotations will be skipped.

This example will match a `pull_request` event targeting the branch `main`
coming from a branch called `wip`:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch == "main" && source_branch == "wip"
```

The fields available are :

| **Field**         | **Description**                                                                                                                  |
|-------------------|----------------------------------------------------------------------------------------------------------------------------------|
| `event`           | `push`, `pull_request` or `incoming`.                                                                                            |
| `target_branch`   | The branch we are targeting.                                                                                                     |
| `source_branch`   | The branch where this pull_request comes from. (On `push`, this is the same as `target_branch`.)                                 |
| `target_url`      | The URL of the repository we are targeting.                                                                                      |
| `source_url`      | The URL of the repository where this pull_request comes from. (On `push`, this is the same as `target_url`.)                     |
| `event_title`     | Matches the title of the event. For `push`, it matches the commit title. For PR, it matches the Pull/Merge Request title. (Only supported for `GitHub`, `Gitlab`, and `BitbucketCloud` providers.) |
| `body`            | The full body as passed by the Git provider. Example: `body.pull_request.number` retrieves the pull request number on GitHub.    |
| `headers`         | The full set of headers as passed by the Git provider. Example: `headers['x-github-event']` retrieves the event type on GitHub.  |
| `.pathChanged`    | A suffix function to a string that can be a glob of a path to check if changed. (Supported only for `GitHub` and `Gitlab` providers.) |
| `files`           | The list of files that changed in the event (`all`, `added`, `deleted`, `modified`, and `renamed`). Example: `files.all` or `files.deleted`. For pull requests, every file belonging to the pull request will be listed. |

CEL expressions lets you do more complex filtering compared to the simple `on-target` annotation matching and enable more advanced scenarios.

For example if I want to have a `PipelineRun` targeting a `pull_request` but
not the `experimental` branch you could have :

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch != experimental"
```

{{< hint info >}}
You can find more information about the CEL language spec here :

<https://github.com/google/cel-spec/blob/master/doc/langdef.md>
{{< /hint >}}

### Matching a PipelineRun to a branch with a regexp

In a CEL expression, you can match a field name using a regular expression. For
example if you want to trigger a `PipelineRun` for the`pull_request` event and
the `source_branch` name containing the substring `feat/`.  you can use the
following expression:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && source_branch.matches(".*feat/.*")
```

### Matching PipelineRun by path change

> *NOTE*: `Pipelines-as-Code` supports two ways to match files changed in a particular event. The `.pathChanged` suffix function supports [glob
pattern](https://github.com/gobwas/glob#example) and does not support different types of "changes" i.e. added, modified, deleted and so on. The other option is to use the `files.` property (`files.all`, `files.added`, `files.deleted`, `files.modified`, `files.renamed`) which can target specific types of changed files and supports using CEL expressions i.e. `files.all.exists(x, x.matches('renamed.go'))`.

If you want to have a PipelineRun running only if a path has
changed you can use the `.pathChanged` suffix function with a [glob
pattern](https://github.com/gobwas/glob#example). Here
is a concrete example matching every markdown files (as files who has the `.md`
suffix) in the `docs` directory :

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && "docs/*.md".pathChanged()
```

This example will match any changed file (added, modified, removed or renamed) that was in the `tmp` directory:

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      files.all.exists(x, x.matches('tmp/'))
```

This example will match any added file that was in the `src` or `pkg` directory:

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      files.added.exists(x, x.matches('src/|pkg/'))
```

This example will match modified files with the name of test.go:

```yaml
    pipelinesascode.tekton.dev/on-cel-expression: |
      files.modified.exists(x, x.matches('test.go'))
```

### Matching PipelineRun to a event (commit, pull_request) title

This example will match all pull request starting with the title `[DOWNSTREAM]`:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request && event_title.startsWith("[DOWNSTREAM]")
```

The event title will be the pull request title on `pull_request` and the
commit title on `push`

### Matching PipelineRun on body payload

{{< tech_preview "Matching PipelineRun on body payload" >}}

The payload body as passed by the Git provider is available in the CEL
variable as `body` and you can use this expression to do any filtering on
anything the Git provider is sending over:

For example this expression when run on GitHub:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  body.pull_request.base.ref == "main" &&
    body.pull_request.user.login == "superuser" &&
    body.action == "synchronize"
```

will only match if the pull request is targeting the `main` branch, the author
of the pull request is called `superuser` and the action is `synchronize` (ie:
an update occurred on a pull request)

{{< hint info >}}
When matching the body payload in a Pull Request, the GitOps comments such as
`/retest` won't be working as expected.

The payload body will become of the comment and not the original pull request
payload.

Consequently, when a pull request event occurs, like opening or updating a pull
request, the CEL body payload may not align with the defined specifications.

To be able to retest your Pull Request when using a CEL on bod payload,
you can make a dummy update to the Pull Request by sending a new SHA with this
git command:

```bash
# assuming you are on the branch you want to retest
# and the upstream remote are set
git commit --amend --no-edit && \
  git push --force-with-lease
```

or close/open the pull request.

{{< /hint >}}

### Matching PipelineRun on request header

You can do some further filtering on the headers as passed by the Git provider
with the CEL variable `headers`.

The headers are available as a list and are always in lower case.

For example this is how to make sure the event is a pull_request on [GitHub](https://docs.github.com/en/webhooks/webhook-events-and-payloads#delivery-headers):

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  headers['x-github-event'] == "pull_request"
```

## Matching a PipelineRun to a branch with comma

If you want to match multiple branches but one branch has a comma in there you
will not be able to match it. In that case you can use the html escape entity
`&#44;` as comma in the name of the branch, for example if you want to match
main and the branch called `release,nightly` you can do this:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release&#44;nightly]
```
