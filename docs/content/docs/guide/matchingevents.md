---
title: Matching a PipelineRun
weight: 3
---

# Matching a PipelineRun to a Git provider Event

A `PipelineRun` can be matched to a Git provider event by using specific
annotations in the `PipelineRun` metadata.

For example, when you have these as metadata in your `PipelineRun`:

```yaml
metadata:
  name: pipeline-pr-main
annotations:
  pipelinesascode.tekton.dev/on-target-branch: "[main]"
  pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines-as-Code` will match the PipelineRun `pipeline-pr-main` if the Git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branches can be specified, separated by commas, e.g.:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release-nightly]
```

You can match on `pull_request` events as above, and you can also match
PipelineRuns on `push` events to a repository.

For example, this will match the pipeline when there is a push to a commit in the
`main` branch:

```yaml
metadata:
  name: pipeline-push-on-main
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/heads/main]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

You can specify the full refs like `refs/heads/main` or the short ref like
`main`. You can also specify globs, for example, `refs/heads/*` will match any
target branch or `refs/tags/1.*` will match all the tags starting from `1.`.

A full example for a push of a tag:

```yaml
metadata:
name: pipeline-push-on-1.0-tags
annotations:
  pipelinesascode.tekton.dev/on-target-branch: "[refs/tags/1.0]"
  pipelinesascode.tekton.dev/on-event: "[push]"
```

This will match the pipeline `pipeline-push-on-1.0-tags` when you push the 1.0
tags into your repository.

{{< hint warning >}}
GitHub does not send webhook events when more than three tags are pushed simultaneously (e.g., with `git push origin --tags`). To ensure pipeline runs are triggered for all tags, push them in batches of three or fewer. [See GitHub's docs here](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#create).
{{< /hint >}}

Matching annotations are currently required; otherwise, Pipelines-as-Code will not
match your `PipelineRun`.

When multiple PipelineRuns match an event, it will run them in parallel
and post the results to the provider as soon as the PipelineRun finishes.

{{< hint info >}}

* Payload matching happens only for events supported by `Pipelines-as-Code`,
such as when a `Pull Request` is opened, updated, or when a branch receives a
`Push`.

* Typically, you need both `on-target-branch` and `on-event` annotations to
match, except when using [CEL expressions](#advanced-event-matching-using-cel)
or [matching based on a
comment](#matching-a-pipelinerun-on-a-regex-in-a-comment).
{{< /hint >}}

## Matching a PipelineRun to Specific Path Changes

{{< tech_preview "Matching a PipelineRun to specific path changes via annotation" >}}

To trigger a `PipelineRun` based on specific path changes in an event, use the
annotation `pipelinesascode.tekton.dev/on-path-change`.

Multiple paths can be specified, separated by commas. The first glob matching
the files changes in the PR will trigger the `PipelineRun`. If you want to match
a file or path that has a comma, you can HTML escape it with the `&#44;` HTML
entity.

You still need to specify the event type and target branch. If you have a [CEL
expression](#matching-pipelinerun-by-path-change) the `on-path-change`
annotation will be ignored.

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
patterns, not regex. Here are some
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
annotation will be ignored.

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
`on-path-change` annotation. It means if you have these annotations:

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

## Matching a PipelineRun on a Regex in a comment

{{< tech_preview "Matching PipelineRun on regex in comments" >}}
{{< support_matrix github_app="true" github_webhook="true" gitea="true" gitlab="true" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

You can trigger a PipelineRun based on a comment on a Pull Request or a [Pushed
Commit]({{< relref
"/docs/guide/running.md#gitops-commands-on-pushed-commits">}}) using the
annotation `pipelinesascode.tekton.dev/on-comment`.

The comment is treated as a regular expression (regex). The spaces and newlines
are stripped at the beginning or the end of the comment before matching so `^`
will match the beginning of the comment and `$` will match the end of the
comment without newlines or space.

If a new comment on a Pull Request matches the specified regex, the PipelineRun
will be triggered and started. This only applies to newly created comments;
updates or edits to existing comments will not trigger the PipelineRun.

Example:

```yaml
---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: "merge-pr"
  annotations:
    pipelinesascode.tekton.dev/on-comment: "^/merge-pr"
```

This will trigger the merge-pr PipelineRun when a comment on a pull request
starts with `/merge-pr`.

When a PipelineRun is getting triggered by the `on-comment` annotation starts,
the template variable {{ trigger_comment }} is set. For more details, refer to
the [documentation]({{< relref
"/docs/guide/gitops_commands.md#accessing-the-comment-triggering-the-pipelinerun"
>}}).

Note that the on-comment annotation adheres to the pull_request [Policy]({{<
relref "/docs/guide/policy" >}}) rule. Only users specified in the pull_request
policy will be able to trigger the PipelineRun.

{{< hint info >}}
The on-comment annotation is supported for pull_request events. For push events,
it is only supported [when targeting the main branch without arguments]({{<
relref "/docs/guide/gitops_commands.md#gitops-commands-on-pushed-commits" >}}).
{{< /hint >}}

## Matching PipelineRun to a Pull Request labels

{{< tech_preview "Matching PipelineRun to a Pull-Request label" >}}
{{< support_matrix github_app="true" github_webhook="true" gitea="true" gitlab="true" bitbucket_cloud="false" bitbucket_datacenter="false" >}}

Using the annotation `pipelinesascode.tekton.dev/on-label`, you can match a
PipelineRun to a Pull Request label. For example, if you want to match the
PipelineRun `bugs` whenever a Pull Request has the label `bug` or `defect`, you
can use this annotation:

```yaml
metadata:
  name: match-bugs-or-defect
  annotations:
    pipelinesascode.tekton.dev/on-label: "[bug, defect]"
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

* The `on-label` annotation respects the `pull_request` [Policy]({{< relref
  "/docs/guide/policy" >}}) rules.
* The `on-target-branch` is still needed to match the Pull Request event on the
  targeted branch.
* The `on-event` is still needed to match the Pull Request event on the
  proper targeted event.
* This annotation is currently supported only on GitHub, Gitea, and GitLab
  providers. Bitbucket Cloud and Bitbucket Data Center do not support adding labels
  to Pull Requests.
* When you add a label to a Pull Request, the corresponding PipelineRun is
  triggered immediately, and no other PipelineRun matching the same Pull Request
  will be activated.
* If you update the Pull Request by sending a new commit, the PipelineRun
  with a matching `on-label` annotation will be triggered again if the label is
  still present.
* You can access the `Pull Request` labels with the [dynamic variable]({{<
  relref "/docs/guide/authoringprs#dynamic-variables" >}}) `{{ pull_request_labels }}`.
  The labels are separated by a Unix newline `\n`.
  For example, with a shell script, you can do this to print them:

  ```bash
   for i in $(echo -e "{{ pull_request_labels }}");do
   echo $i
   done
  ```

## Advanced event matching using CEL

If you need to do some advanced matching, `Pipelines-as-Code` supports CEL
expressions to do advanced filtering on the specific event you need to be matched.

{{< hint danger >}}
If you use the `on-cel-expression` annotation in the same PipelineRun as an `on-event`, `on-target-branch`, `on-label`, `on-path-change`, or `on-path-change-ignore`
annotation, the `on-cel-expression` annotation takes priority and Pipelines-as-Code ignores the other annotations.
{{< /hint >}}

This example will match a `pull_request` event targeting the branch `main`
coming from a branch called `wip`:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch == "main" && source_branch == "wip"
```

The fields available are:

| **Field** | **Description** |
| --- | --- |
| `event` | `push`, `pull_request` or `incoming`. |
| `event_type` | The event type from the webhook payload header. Provider-specific (e.g., GitHub sends `pull_request`, GitLab is `Merge Request`, etc). |
| `target_branch` | The branch we are targeting. |
| `source_branch` | The branch where this pull_request comes from. (On `push`, this is the same as `target_branch`.) |
| `target_url` | The URL of the repository we are targeting. |
| `source_url` | The URL of the repository where this pull_request comes from. (On `push`, this is the same as `target_url`.) |
| `event_title` | Matches the title of the event. For `push`, it matches the commit title. For PR, it matches the Pull/Merge Request title. (Only supported for `GitHub`, `GitLab`, and `BitbucketCloud` providers.) |
| `body` | The full body as passed by the Git provider. Example: `body.pull_request.number` retrieves the pull request number on GitHub. |
| `headers` | The full set of headers as passed by the Git provider. Example: `headers['x-github-event']` retrieves the event type on GitHub. |
| `.pathChanged` | A suffix function to a string that can be a glob of a path to check if changed. (Supported only for `GitHub` and `GitLab` providers.) |
| `files` | The list of files that changed in the event (`all`, `added`, `deleted`, `modified`, and `renamed`). Example: `files.all` or `files.deleted`. For pull requests, every file belonging to the pull request will be listed. |
| Custom params | Any [custom parameters]({{< relref "/docs/guide/customparams" >}}) provided from the Repository CR `spec.params` are available as CEL variables. Example: `enable_ci == "true"`. See [Using custom parameters in CEL expressions: limitations](#using-custom-parameters-in-cel-expressions-limitations) below for important details. |

CEL expressions let you do more complex filtering compared to the simple `on-target` annotation matching and enable more advanced scenarios.

For example, if you want to have a `PipelineRun` targeting a `pull_request` but
not the `experimental` branch you could have:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && target_branch != "experimental"
```

{{< hint info >}}
You can find more information about the CEL language spec here:

<https://github.com/google/cel-spec/blob/master/doc/langdef.md>
{{< /hint >}}

### Using custom parameters in CEL expressions: limitations

#### Filtered custom parameters and CEL evaluation

When using a custom parameter with a `filter` in a CEL expression, be aware that if the filter condition
is **not met**, the parameter will be **undefined**, causing a CEL evaluation error rather than evaluating to false.

For example, consider this Repository CR:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: my-repo
spec:
  url: "https://github.com/owner/repo"
  params:
    - name: docker_registry
      value: "registry.staging.example.com"
      filter: pac.event_type == "pull_request"
```

And this PipelineRun:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: my-pipeline
  annotations:
    pipelinesascode.tekton.dev/on-cel-expression: |
      docker_registry == "registry.staging.example.com"
spec:
  # ... pipeline spec
```

On a **push event**, the `docker_registry` parameter will not be defined (since the filter only matches pull
requests), and the CEL expression will produce an **error**, not `false`. The PipelineRun will not be
evaluated and an error will be reported.

To avoid undefined parameter errors, ensure your CEL expressions only reference custom parameters when their
filter conditions match, or use parameters without filters for CEL matching. We recommend testing your CEL
expressions with different event types using the [tkn pac cel]({{< relref "/docs/guide/cli#tkn-pac-cel" >}})
command to verify they work correctly across all scenarios

#### Custom parameters do not override standard CEL variables

Custom parameters defined in the Repository CR cannot override the built-in CEL variables provided by
Pipelines-as-Code, such as:

* `event` (or `event_type`)
* `target_branch`
* `source_branch`
* `trigger_target`
* And other default variables documented in the table above

If you define a custom parameter with the same name as a standard CEL variable, the standard variable will
take precedence in CEL expressions. Custom parameters should use unique names that don't conflict with
built-in variables.

### Matching a PipelineRun to a branch with a regex

In a CEL expression, you can match a field name using a regular expression. For
example, if you want to trigger a `PipelineRun` for the`pull_request` event and
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
is a concrete example matching every markdown file (as files that have the `.md`
suffix) in the `docs` directory:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && "docs/*.md".pathChanged()
```

This example will match any changed file (added, modified, removed, or renamed) that was in the `tmp` directory:

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

### Filtering PipelineRuns to exclude non-code changes

This example demonstrates how to filter `pull_request` events to exclude changes that only affect documentation, configuration files, or other non-code files. This is useful when you want to run tests only when actual code changes occur:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request"
  && target_branch == "main"
  && !files.all.all(x, x.matches('^docs/') || x.matches('\\.md$') || x.matches('(\\.gitignore|OWNERS|PROJECT|LICENSE)$'))
```

This expression will:

* Only match `pull_request` events targeting the `main` branch
* **Exclude** the PipelineRun if all changed files match any of the following patterns:
  * Files in the `docs/` directory (`^docs/`)
  * Markdown files (`\\.md$`)
  * Common repository metadata files (`\\.gitignore`, `OWNERS`, `PROJECT`, `LICENSE`)

The `!files.all.all(x, x.matches('pattern1') || x.matches('pattern2') || ...)` syntax means "not all files match any of these patterns", which effectively means "trigger only if at least one file doesn't match the exclusion patterns" (i.e., there are meaningful code changes).

{{< hint warning >}}
**Important**: When using regex patterns in CEL expressions, remember to properly escape special characters. The backslash (`\`) needs to be doubled (`\\`) to escape properly within the CEL string context. Using logical OR (`||`) operators within the `matches()` function is more reliable than combining patterns with pipe (`|`) characters in a single regex.
{{< /hint >}}

### Matching PipelineRun to an event (commit, pull_request) title

This example will match all pull requests starting with the title `[DOWNSTREAM]`:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  event == "pull_request" && event_title.startsWith("[DOWNSTREAM]")
```

The event title will be the pull request title on `pull_request` and the
commit title on `push`.

### Matching PipelineRun on body payload

{{< tech_preview "Matching PipelineRun on body payload" >}}

The payload body as passed by the Git provider is available in the CEL
variable as `body` and you can use this expression to do any filtering on
anything the Git provider is sending over:

For example, this expression when run on GitHub:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  body.pull_request.base.ref == "main" &&
    body.pull_request.user.login == "superuser" &&
    body.action == "synchronize"
```

will only match if the pull request is targeting the `main` branch, the author
of the pull request is called `superuser` and the action is `synchronize` (i.e.:
an update occurred on a pull request).

{{< hint info >}}
When matching the body payload in a Pull Request, the GitOps comments such as
`/retest` won't be working as expected.

The payload body will become of the comment and not the original pull request
payload.

Consequently, when a pull request event occurs, like opening or updating a pull
request, the CEL body payload may not align with the defined specifications.

To be able to retest your Pull Request when using a CEL on body payload,
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

### Matching a PipelineRun to a request header

You can do some further filtering on the headers as passed by the Git provider
with the CEL variable `headers`.

The headers are available as a list and are always in lower case.

For example, this is how to make sure the event is a pull_request on [GitHub](https://docs.github.com/en/webhooks/webhook-events-and-payloads#delivery-headers):

```yaml
pipelinesascode.tekton.dev/on-cel-expression: |
  headers['x-github-event'] == "pull_request"
```

## Matching a PipelineRun to a branch with a comma

If you want to match multiple branches but one branch has a comma in there you
will not be able to match it. In that case, you can use the HTML escape entity
`&#44;` as a comma in the name of the branch, for example, if you want to match
main and the branch called `release,nightly` you can do this:

```yaml
pipelinesascode.tekton.dev/on-target-branch: [main, release&#44;nightly]
```

## Skip CI Commands

Pipelines-as-Code supports skip commands in commit messages that allow you to skip
PipelineRun execution for specific commits. This is useful when making documentation
changes, minor fixes, or work-in-progress commits where running the full CI pipeline
is unnecessary.

### Supported Skip Commands

You can include any of the following commands anywhere in your commit message to skip
PipelineRun execution:

* `[skip ci]` - Skip continuous integration
* `[ci skip]` - Alternative format for skipping CI
* `[skip tkn]` - Skip Tekton PipelineRuns
* `[tkn skip]` - Alternative format for skipping Tekton

**Note:** Skip commands are **case-sensitive** and must be in lowercase with brackets.

### Example Usage

```text
docs: update README with installation instructions [skip ci]
```

or

```text
WIP: refactor authentication module

This is still in progress and not ready for testing yet.

[ci skip]
```

### How Skip Commands Work

When a commit message contains a skip command:

1. **Pull Requests**: No PipelineRuns will be created when the PR is opened or updated and HEAD commit contains skip command. A neutral status check will be displayed on the PR indicating that CI was skipped.
2. **Push Events**: No PipelineRuns will be created when pushing to a branch with that commit message. A neutral status check will be displayed on the commit.

**Note:** A neutral status check is created on your git provider to provide visibility that the commit was acknowledged but CI was intentionally skipped. This helps distinguish between commits that were ignored due to skip commands versus commits where CI hasn't run.

### GitOps Commands Override Skip CI

**Important:** Skip CI commands can be overridden by using GitOps commands. Even if
a commit contains a skip command like `[skip ci]`, you can still manually trigger
PipelineRuns using:

* `/test` - Trigger all matching PipelineRuns
* `/test <pipelinerun-name>` - Trigger a specific PipelineRun
* `/retest` - Retrigger failed PipelineRuns
* `/retest <pipelinerun-name>` - Retrigger a specific PipelineRun
* `/ok-to-test` - Allow running CI for external contributors
* `/custom-comment` - Trigger PipelineRun having on-comment annotation

This allows you to skip automatic CI execution while still maintaining the ability
to manually trigger builds when needed.

### Example: Skipping CI Then Manually Triggering

```bash
# Initial commit with skip command
git commit -m "docs: update contributing guide [skip ci]"
git push origin my-feature-branch
# No PipelineRuns are created automatically
# A neutral status check is displayed on the commit/PR

# Later, you can manually trigger CI by commenting on the PR:
# /test
# This will create PipelineRuns despite the [skip ci] command
```

### Examples of When to Use Skip Commands

Skip commands are useful for:

* Documentation-only changes
* README updates
* Comment or formatting changes
* Work-in-progress commits
* Minor typo fixes
* Configuration file updates that don't affect code

### Examples of When NOT to Use Skip Commands

Avoid using skip commands for:

* Code changes that affect functionality
* Changes to CI/CD pipeline definitions
* Dependency updates
* Any changes that should be tested before merging
