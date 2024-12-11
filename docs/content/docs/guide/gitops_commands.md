---
title: Gitops Commands
weight: 5.1
---
# GitOps commands

Pipelines-as-Code support the concept of `GitOps commands` a way to have the
users issue a special command on a comment of a Pull Request or a Pushed Commit
to control `Pipelines-as-Code`.

The advantage of using a `Gitops command` is to get a journal of all the
execution of your Pipeline directly on your Pull request near your code.

## GitOps commands on Pull Requests

For example when you are on a Pull Request you may want to restart all your
pipelineruns, to do so you can add a comment on your pull request starting with
`/retest` and all PipelineRuns attached to that pull request will be restarted :

Example :

```text
Thanks for contributing, This is a much needed bugfix, and we love it ❤️ The
failure is not with your PR but seems to be an infra issue.

/retest
```

If you have multiple `PipelineRun` and you want to target a specific
`PipelineRun` you can use the `/test` and the specific PipelineRun as a comment
to restart it, example:

```text
roses are red, violets are blue. pipeline are bound to flake by design.

/test <pipelinerun-name>
```

## GitOps commands on pushed commits

If you want to trigger a GitOps command on a pushed commit, you can include the
`GitOps` comments within your commit messages. These comments can be used to
restart either all pipelines or specific ones. Here's how it works:

For restarting all pipeline runs:

1. Use `/retest` or `/test` within your commit message.

For restarting a specific pipeline run:
2. Use `/retest <pipelinerun-name>` or `/test <pipelinerun-name>` within your
commit message. Replace `<pipelinerun-name>` with the specific name of the
pipeline run you want to restart.

**Note:**

When executing `GitOps` commands on a commit that exists in multiple branches
within a push request, the branch with the latest commit will be used.

This means:

1. If a user specifies commands like `/retest` or `/test` without any argument
in a comment on a branch, the test will automatically be performed on the **main** branch.

   Examples :
   1. `/retest`
   2. `/test`
   3. `/retest <pipelinerun-name>`
   4. `/test <pipelinerun-name>`

2. If the user includes a branch specification such as `/retest branch:test` or
`/test branch:test`, the test will be executed on the commit where the comment is
located, with the context of the **test** branch.

   Examples :
   1. `/retest branch:test`
   2. `/test branch:test`
   3. `/retest <pipelinerun-name> branch:test`
   4. `/test <pipelinerun-name> branch:test`

To issue a `GitOps` comment on a pushed commit you can follow these steps:

1. Go to your repository.
2. Click on the **Commits** section.
3. Choose one of the individual **Commit**.
4. Click on the line number where you want to add a `GitOps` comment, as shown in the image below:

![GitOps Commits For Comments](/images/gitops-comments-on-commit.png)

Please note that this feature is supported for the GitHub provider only.

## GitOps commands on non-matching PipelineRun

The PipelineRun will be restarted regardless of the annotations if the comment
`/test <pipelinerun-name>` or `/retest <pipelinerun-name>` is used . This let
you have control of PipelineRuns that gets only triggered by a comment on the
pull request.

## Accessing the comment triggering the PipelineRun

When you trigger a PipelineRun via a Gitops Command, the template variable `{{
trigger_comment }}` get set to the actual comment that triggered it.

You can then do some action based on for example the comment content with a
shell script or others.

There is a restriction with the `trigger_comment` variable, we modify it to
replace the newline with a `\n` since the multi-line comment can cause a issue
when replaced inside the yaml.

It is up to you to replace it back with newlines, for example with shell scripts
you can use `echo -e` to expand the newline back.

Example of a shell script:

```shell
echo -e "{{ trigger_comment }}" > /tmp/comment
grep "string" /tmp/comment
```

## Custom GitOps commands

Using the [on-comment]({{< relref
"/docs/guide/matchingevents.md#matching-a-pipelinerun-on-a-regexp-in-a-comment"
>}}) annotation on your `PipelineRun` you can define custom GitOps commands that
will be triggered by the comments on the pull request.

See the [on-comment]({{< relref
"/docs/guide/matchingevents.md#matching-a-pipelinerun-on-a-regexp-in-a-comment"
>}}) guide for more detailed information.

## Cancelling a PipelineRun

You can cancel a running PipelineRun by commenting on the PullRequest.

For example if you want to cancel all your PipelinerRuns you can add a comment starting
with `/cancel` and all PipelineRun attached to that pull request will be cancelled.

Example :

```text
It seems the infra is down, so cancelling the pipelineruns.

/cancel
```

If you have multiple `PipelineRun` and you want to target a specific
`PipelineRun` you can use the `/cancel` comment with the PipelineRun name

Example :

```text
roses are red, violets are blue. why to run the pipeline when the infra is down.

/cancel <pipelinerun-name>
```

On GitHub App the status of the Pipeline will be set to `cancelled`.

![pipelinerun canceled](/images/pr-cancel.png)

### Cancelling the PipelineRun on push request

You can cancel a running PipelineRun by commenting on the commit.
Here's how you can do it.

Example :

1. Use `/cancel` to cancel all PipeineRuns.
2. Use `/cancel <pipelinerun-name>` to cancel a specific PipeineRun

**Note:**

When executing `GitOps` comments on a commit that exists in multiple branches
within a push request, the branch with the latest commit will be used.

This means:

1. If a user specifies commands like `/cancel`
without any argument in a comment on a branch,
it will automatically target the **main** branch.

   Examples :
   1. `/cancel`
   2. `/cancel <pipelinerun-name>`

2. If the user issues a command like `/cancel branch:test`,
it will target the commit where the comment was made but use the **test** branch.

   Examples :
   1. `/cancel branch:test`
   2. `/cancel <pipelinerun-name> branch:test`

In the GitHub App, the status of the Pipeline will be set to `cancelled`.

![GitOps Commits For Comments For PipelineRun Canceled](/images/gitops-comments-on-commit-cancel.png)

Please note that this feature is supported for the GitHub provider only.

## Passing parameters to GitOps commands as argument

{{< tech_preview "Passing parameters to GitOps commands as argument" >}}

When you issue a GitOps command, you can pass arguments to the command to
redefine some the [standard]({{< relref "/docs/guide/authoringprs#dynamic-variables" >}})
dynamic variables or the [custom parameters]({{< relref "/docs/guide/customparams" >}})

For example you can do:

```text
/test pipelinerun1 key=value
```

and the custom parameter `key` if defined as custom parameter will be defined to `value`

If the comment does not start with a `/` it will not be parsed.

You can only override parameters from the standard or when set as custom
parameters, you cannot pass arbitrary parameters that hasn't been defined
previously.

You can pass those `key=value` anywhere in your comment and it will be parsed.

There is different format that can get accepted, which let you pass values with space or newlines:

* key=value
* key="a value"
* key="another \"value\" defined"
* key="another
  value with newline"

## Event Type Annotation and dynamic variables

The `pipeline.tekton.dev/event-type` annotation indicates the type of GitOps
command that has triggered the PipelineRun.

Here are the possible event types:

* `test-all-comment` : The event is a single `/test` that would test every matched pipelinerun.
* `test-comment` : The event is a `/test <PipelineRun>` comment that would test a specific PipelineRun.
* `retest-all-comment` : The event is a single `/retest` that would retest every matched pipelinerun.
* `retest-comment` : The event is a `/retest <PipelineRun>` that would retest a specific PipelineRun.
* `on-comment`: The event is coming from a  custom comment that would trigger a PipelineRun.
* `cancel-all-comment` : The event is a single `/cancel` that would cancel every matched pipelinerun.
* `cancel-comment` : The event is a `/cancel <PipelineRun>` that would cancel a specific PipelineRun.
* `ok-to-test-comment` : The event is a `/ok-to-test` that would allow running the CI for a unauthorized user.

When using the `{{ event_type }}` [dynamic variable]({{< relref "/docs/guide/authoringprs.md#dynamic-variables" >}}) for the following event types:

* `test-all-comment`
* `test-comment`
* `retest-all-comment`
* `retest-comment`
* `cancel-all-comment`
* `ok-to-test-comment`

The dynamic variable will return `pull_request` as the event type instead of the specific
categorized GitOps command type. This is to handle backward compatibility with
previous release for users relying on this dynamic variable.

This currently only issue a warning in the repository matched namespace but then deprecated and changed to return
the specific event type.
