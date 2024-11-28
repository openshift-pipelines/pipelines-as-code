---
title: Authoring PipelineRun
weight: 3
---

# Authoring PipelineRuns in `.tekton/` directory

- Pipelines-as-Code will always try to be as close to the tekton template as
  possible. Usually you will write your template and save them with a `.yaml`
  extension and Pipelines-as-Code will run them.

- The `.tekton` directory must be at the top level of the repo.
  You can reference YAML files in other repos using remote URLs
  (see [Remote HTTP URLs](./resolver.md#remote-http-url) for more information),
  but PipelineRuns will only be triggered by events in the repository containing
  the `.tekton` directory.

- Using its [resolver](../resolver/) Pipelines-as-Code will try to bundle the
  PipelineRun with all its Task as a single PipelineRun with no external
  dependencies.

- Inside your pipeline you need to be able to check out the commit as
  received from the webhook by checking it out the repository from that ref. Most of the time
  you want to reuse the
  [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from the [tektoncd/catalog](https://github.com/tektoncd/catalog).

- To be able to specify parameters of your commit and URL, Pipelines-as-Code
  give you some “dynamic” variables that is defined according to the execution
  of the events. Those variables look like this `{{ var }}` and can be used
  anywhere in your template, see [below](#dynamic-variables) for the list of
  available variables.

- For Pipelines-as-Code to process your `PipelineRun`, you must have either an
  embedded `PipelineSpec` or a separate `Pipeline` object that references a YAML
  file in the `.tekton` directory. The Pipeline object can include `TaskSpecs`,
  which may be defined separately as Tasks in another YAML file in the same
  directory. It's important to give each `PipelineRun` a unique name to avoid
  conflicts. **PipelineRuns with duplicate names will never be matched**.

## Dynamic variables

Here is a list of al the dynamic variables available in Pipelines-as-Code. The
one that would be the most important to you would probably be the `revision` and `repo_url`
variables, they will give you the commit SHA and the repository URL that is
getting tested. You usually use this with the
[git-clone](https://hub.tekton.dev/tekton/task/git-clone) task to be able to
checkout the code that is being tested.

| Variable            | Description                                                                                       | Example                             | Example Output               |
|---------------------|---------------------------------------------------------------------------------------------------|-------------------------------------|------------------------------|
| body                | The full payload body (see [below](#using-the-body-and-headers-in-a-pipelines-as-code-parameter)) | `{{body.pull_request.user.email }}` | <email@domain.com>           |
| event_type          | The event type (eg: `pull_request` or `push`)                                                     | `{{event_type}}`                    | pull_request          (see the note for Gitops Comments [here]({{< relref "/docs/guide/gitops_commands.md#event-type-annotation-and-dynamic-variables" >}}) )     |
| git_auth_secret     | The secret name auto generated with provider token to check out private repos.                    | `{{git_auth_secret}}`               | pac-gitauth-xkxkx            |
| headers             | The request headers (see [below](#using-the-body-and-headers-in-a-pipelines-as-code-parameter))   | `{{headers['x-github-event']}}`     | push                         |
| pull_request_number | The pull or merge request number, only defined when we are in a `pull_request` event type.        | `{{pull_request_number}}`           | 1                            |
| repo_name           | The repository name.                                                                              | `{{repo_name}}`                     | pipelines-as-code            |
| repo_owner          | The repository owner.                                                                             | `{{repo_owner}}`                    | openshift-pipelines          |
| repo_url            | The repository full URL.                                                                          | `{{repo_url}}`                      | https:/github.com/repo/owner |
| revision            | The commit full sha revision.                                                                     | `{{revision}}`                      | 1234567890abcdef             |
| sender              | The sender username (or accountid on some providers) of the commit.                               | `{{sender}}`                        | johndoe                      |
| source_branch       | The branch name where the event come from.                                                        | `{{source_branch}}`                 | main                         |
| source_url          | The source repository URL from which the event come from (same as `repo_url` for push events).    | `{{source_url}}`                    | https:/github.com/repo/owner |
| target_branch       | The branch name on which the event targets (same as `source_branch` for push events).             | `{{target_branch}}`                 | main                         |
| target_namespace    | The target namespace where the Repository has matched and the PipelineRun will be created.        | `{{target_namespace}}`              | my-namespace                 |
| trigger_comment     | The comment triggering the pipelinerun when using a [GitOps command]({{< relref "/docs/guide/running.md#gitops-command-on-pull-or-merge-request" >}}) (like `/test`, `/retest`)      | `{{trigger_comment}}`               | /merge-pr branch             |

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

`Pipelines-as-Code` will match the pipelinerun `pipeline-pr-main` if the Git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branch can be specified separated by comma, i.e:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release-nightly]
```

If you want to match a branch that has a comma (,) in its name you can html escape entity
`&#44;` as comma, for example if you want to match main and the branch
called `release,nightly` you can do this:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release&#44;nightly]
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

Matching annotations are currently mandated or `Pipelines-as-Code` will not
match your `PipelineRun`.

If there are multiple pipelinerun matching an event, it will run all of them in
parallel and posting the results to the provider as soon the PipelineRun
finishes.

{{< hint info >}}
The matching on payload can only occur on the events Pipelines-as-Code responds
too, it will only be matched when a `Pull Request` is opened or updated or on a
`Push` to a branch
{{< /hint >}}

### Matching a PipelineRun to Specific Path Changes

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

The `tkn pac` cli has a handy [globbing command]({{< relref "/docs/guide/cli.md#test-globbing-pattern" >}})
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

## Advanced event matching

If you need to do some advanced matching, `Pipelines-as-Code` supports CEL
filtering.

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

- `event`: `push` or `pull_request`
- `target_branch`: The branch we are targeting.
- `source_branch`: The branch where this pull_request come from. (on `push` this
  is the same as `target_branch`).
- `target_url`: The URL of the repository we are targeting.
- `source_url`: The URL of the repository where this pull_request come from. (on `push` this
  is the same as `target_url`).
- `event_title`: Match the title of the event. When doing a push this will match
  the commit title and when matching on PR it will match the Pull or Merge
  Request title. (only `GitHub`, `Gitlab` and `BitbucketCloud` providers are supported)
- `body`: The full body as passed by the Git provider. (example: `body.pull_request.number` will get the pull request number on GitHub)
- `headers`: The full set of headers as passed by the Git provider. (example: `headers['x-github-event']` will get the event type on GitHub)
- `.pathChanged`: a suffix function to a string which can be a glob of a path to
  check if changed (only `GitHub` and `Gitlab` provider is supported)
- `files`: The list of files that changed in the event (all, added, deleted, modified and renamed). Example `files.all` or `files.deleted`. On pull request every file belonging to the pull request will be listed.

Compared to the simple "on-target" annotation matching, the CEL expression
allows you to complex filtering and most importantly express negation.

For example if I want to have a `PipelineRun` targeting a `pull_request` but
not the `experimental` branch I would have :

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch != experimental"
```

## Matching a PipelineRun on a regexp in CEL language

In CEL expression, you can match a field name using a regular expression.
For example, you want to trigger a `PipelineRun` when event is `pull_request` and the `source_branch` name contains the substring `feat/`.
you can use the following expression:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && source_branch.matches(".*feat/.*")
```

You can find more information about the CEL language spec here :

<https://github.com/google/cel-spec/blob/master/doc/langdef.md>

### Matching a PipelineRun on a regexp in a comment

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

Will match the merge-pr PipelineRun when a comment on a pull request starts
with `/merge-pr`

When the PipelineRun that has been triggered with the `on-comment` annotation
gets started the template variable `{{ trigger_comment }}` get set. See the
documentation [here]({{< relref "/docs/guide/gitops_commands.md#accessing-the-comment-triggering-the-pipelinerun" >}})

Note that the `on-comment` annotation will respect the `pull_request` [Policy]({{< relref "/docs/guide/policy" >}}) rule,
so only users into the `pull_request` policy will be able to trigger the
PipelineRun.

> *NOTE*: The `on-comment` annotation is only supported on GitHub, Gitea and GitLab providers

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

### Matching PipelineRun on event title

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

## Using the body and headers in a Pipelines-as-Code parameter

Pipelines-as-Code let you access the full body and headers of the request as a CEL expression.

This allows you to go beyond the standard variables and even play with multiple
conditions and variable to output values.

For example if you want to get the title of the Pull Request in your PipelineRun you can simply access it like this:

```go
{{ body.pull_request.title }}
```

You can then get creative and for example mix the variable inside a python
script to evaluate the json.

This task for example is using python and will check the labels on the PR,
`exit 0` if it has the label called 'bug' on the pull request or `exit 1` if it
doesn't:

```yaml
taskSpec:
  steps:
    - name: check-label
      image: registry.access.redhat.com/ubi9/ubi
      script: |
        #!/usr/bin/env python3
        import json
        labels=json.loads("""{{ body.pull_request.labels }}""")
        for label in labels:
            if label['name'] == 'bug':
              print('This is a PR targeting a BUG')
              exit(0)
        print('This is not a PR targeting a BUG :(')
        exit(1)
```

The expression are CEL expressions so you can as well make some conditional:

```yaml
- name: bash
  image: registry.access.redhat.com/ubi9/ubi
  script: |
    if {{ body.pull_request.state == "open" }}; then
      echo "PR is Open"
    fi
```

if the PR is open the condition then return `true` and the shell script see this
as a valid boolean.

Headers from the payload body can be accessed from the `headers` keyword, note that headers are case sensitive,
for example this will show the GitHub event type for a GitHub event:

```yaml
{{ headers['X-Github-Event'] }}
```

and then you can do the same conditional or access as described above for the `body` keyword.

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
          value: "Pipelines-as-Code IS GREAT!"
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

- On GitHub apps the generated installation token [will be available for 8 hours](https://docs.github.com/en/developers/apps/building-github-apps/refreshing-user-to-server-access-tokens)
- On GitHub apps the token is scoped to the repository the event (payload) come
  from unless [configured](/docs/install/settings#pipelines-as-code-configuration-settings) it differently on cluster.

{{< /hint >}}

## Example

`Pipelines as code` test itself, you can see the examples in its
[.tekton](https://github.com/openshift-pipelines/pipelines-as-code/tree/main/.tekton) repository.
