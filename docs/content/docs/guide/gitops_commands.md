---
title: GitOps Commands
weight: 5.1
---
# GitOps Commands

Pipelines-as-Code supports the concept of `GitOps commands`, which allow users to issue special commands in a comment on a Pull Request or a pushed commit to control `Pipelines-as-Code`.

The advantage of using a `GitOps command` is that it provides a journal of all the executions of your pipeline directly on your Pull Request, near your code.

## GitOps Commands on Pull Requests

For example, when you are on a Pull Request, you may want to restart all your PipelineRuns. To do so, you can add a comment on your Pull Request starting with `/retest`, and all PipelineRuns attached to that Pull Request will be restarted.

Example:

```text
Thanks for contributing. This is a much-needed bugfix, and we love it ❤️ The
failure is not with your PR but seems to be an infrastructure issue.

/retest
```

If you have multiple `PipelineRun` and you want to target a specific `PipelineRun`, you can use the `/test` command followed by the specific PipelineRun name to restart it. Example:

```text
Roses are red, violets are blue. Pipelines are bound to flake by design.

/test <pipelinerun-name>
```

{{< hint info >}}

Please be aware that GitOps commands such as `/test` and others will not function on closed Pull Requests or Merge Requests.  

{{< /hint >}}

## GitOps Commands on Pushed Commits

If you want to trigger a GitOps command on a pushed commit, you can include the `GitOps` comments within your commit messages. These comments can be used to restart either all pipelines or specific ones. Here's how it works:

For restarting all pipeline runs:

1. Use `/retest` or `/test` within your commit message.

For restarting a specific pipeline run:
2. Use `/retest <pipelinerun-name>` or `/test <pipelinerun-name>` within your commit message. Replace `<pipelinerun-name>` with the specific name of the pipeline run you want to restart.

The GitOps command triggers a PipelineRun only on the latest commit (HEAD) of the branch and does not work on older commits.

**Note:**

When executing `GitOps` commands on a commit that exists in multiple branches within a push request, the branch with the latest commit will be used.

This means:

1. When a user comments with commands like `/retest` or `/test` on a branch without specifying a branch name, the test will automatically run on the **default branch** (e.g. main, master) of the repository.

   Examples:
   1. `/retest`
   2. `/test`
   3. `/retest <pipelinerun-name>`
   4. `/test <pipelinerun-name>`

2. If the user includes a branch specification such as `/retest branch:test` or `/test branch:test`, the test will be executed on the commit where the comment is located, with the context of the **test** branch.

   Examples:
   1. `/retest branch:test`
   2. `/test branch:test`
   3. `/retest <pipelinerun-name> branch:test`
   4. `/test <pipelinerun-name> branch:test`

To issue a `GitOps` comment on a pushed commit, you can follow these steps:

1. Go to your repository.
2. Click on the **Commits** section.
3. Choose one of the individual **Commit**.
4. Click on the line number where you want to add a `GitOps` comment, as shown in the image below:

![GitOps Commits For Comments](/images/gitops-comments-on-commit.png)

Please note that this feature is supported for the GitHub provider only.

## GitOps Commands on Non-Matching PipelineRun

The PipelineRun will be restarted regardless of the annotations if the comment `/test <pipelinerun-name>` or `/retest <pipelinerun-name>` is used. This allows you to have control of PipelineRuns that get only triggered by a comment on the Pull Request.

## Accessing the Comment Triggering the PipelineRun

When you trigger a PipelineRun via a GitOps Command, the template variable `{{ trigger_comment }}` is set to the actual comment that triggered it.

You can then perform actions based on the comment content with a shell script or other methods.

There is a restriction with the `trigger_comment` variable: we modify it to replace newlines with `\n` since multi-line comments can cause issues when replaced inside the YAML.

It is up to you to replace it back with newlines. For example, with shell scripts, you can use `echo -e` to expand the newline back.

Example of a shell script:

```shell
echo -e "{{ trigger_comment }}" > /tmp/comment
grep "string" /tmp/comment
```

## Custom GitOps Commands

Using the [on-comment]({{< relref "/docs/guide/matchingevents.md#matching-a-pipelinerun-on-a-regex-in-a-comment" >}}) annotation on your `PipelineRun`, you can define custom GitOps commands that will be triggered by comments on the Pull Request.

See the [on-comment]({{< relref "/docs/guide/matchingevents.md#matching-a-pipelinerun-on-a-regex-in-a-comment" >}}) guide for more detailed information.

For a complete example, you can see how Pipelines-as-Code's own repo implemented some prow comments via the `on-comment` annotation:

<https://github.com/openshift-pipelines/pipelines-as-code/blob/main/.tekton/prow.yaml>

## Cancelling a PipelineRun

You can cancel a running PipelineRun by commenting on the Pull Request.

For example, if you want to cancel all your PipelineRuns, you can add a comment starting with `/cancel`, and all PipelineRuns attached to that Pull Request will be cancelled.

Example:

```text
It seems the infrastructure is down, so cancelling the PipelineRuns.

/cancel
```

If you have multiple `PipelineRun` and you want to target a specific `PipelineRun`, you can use the `/cancel` comment with the PipelineRun name.

Example:

```text
Roses are red, violets are blue. Why run the pipeline when the infrastructure is down.

/cancel <pipelinerun-name>
```

In the GitHub App, the status of the Pipeline will be set to `cancelled`.

![PipelineRun Canceled](/images/pr-cancel.png)

### Cancelling the PipelineRun on Push Request

You can cancel a running PipelineRun by commenting on the commit. Here's how you can do it.

Example:

1. Use `/cancel` to cancel all PipelineRuns.
2. Use `/cancel <pipelinerun-name>` to cancel a specific PipelineRun.

**Note:**

When executing `GitOps` comments on a commit that exists in multiple branches within a push request, the branch with the latest commit will be used.

This means:

1. If a user specifies commands like `/cancel` without any argument in a comment on a branch, it will automatically target the **main** branch.

   Examples:
   1. `/cancel`
   2. `/cancel <pipelinerun-name>`

2. If the user issues a command like `/cancel branch:test`, it will target the commit where the comment was made but use the **test** branch.

   Examples:
   1. `/cancel branch:test`
   2. `/cancel <pipelinerun-name> branch:test`

In the GitHub App, the status of the Pipeline will be set to `cancelled`.

![GitOps Commits For Comments For PipelineRun Canceled](/images/gitops-comments-on-commit-cancel.png)

Please note that this feature is supported for the GitHub provider only.

## Passing Parameters to GitOps Commands as Arguments

{{< tech_preview "Passing parameters to GitOps commands as arguments" >}}

When you issue a GitOps command, you can pass arguments to the command to redefine some of the [standard]({{< relref "/docs/guide/authoringprs#dynamic-variables" >}}) dynamic variables or the [custom parameters]({{< relref "/docs/guide/customparams" >}}).

For example, you can do:

```text
/test pipelinerun1 key=value
```

and the custom parameter `key`, if defined as a custom parameter, will be set to `value`.

If the comment does not start with a `/`, it will not be parsed.

You can only override parameters from the standard or when set as custom parameters; you cannot pass arbitrary parameters that haven't been defined previously.

You can pass those `key=value` pairs anywhere in your comment, and they will be parsed.

There are different formats that can be accepted, allowing you to pass values with spaces or newlines:

* key=value
* key="a value"
* key="another \"value\" defined"
* key="another
  value with newline"

## Event Type Annotation and Dynamic Variables

The `pipeline.tekton.dev/event-type` annotation indicates the type of GitOps command that has triggered the PipelineRun.

Here are the possible event types:

* `test-all-comment`: The event is a single `/test` that would test every matched PipelineRun.
* `test-comment`: The event is a `/test <PipelineRun>` comment that would test a specific PipelineRun.
* `retest-all-comment`: The event is a single `/retest` that would retest every matched PipelineRun.
* `retest-comment`: The event is a `/retest <PipelineRun>` that would retest a specific PipelineRun.
* `on-comment`: The event is coming from a custom comment that would trigger a PipelineRun.
* `cancel-all-comment`: The event is a single `/cancel` that would cancel every matched PipelineRun.
* `cancel-comment`: The event is a `/cancel <PipelineRun>` that would cancel a specific PipelineRun.
* `ok-to-test-comment`: The event is a `/ok-to-test` that would allow running the CI for an unauthorized user.

When a repository owner issues the `/ok-to-test` command on a pull request raised by an unauthorized user, and no PipelineRun exists in the .tekton directory for `pull_request` event,
Pipelines-as-Code will create a neutral check-run status. This status serves to indicate that no PipelineRun has been matched, preventing any workflows from being blocked such as auto-merge, will proceed as expected.

{{< hint info >}}

Note: This neutral check-run status functionality is only supported on GitHub.

{{< /hint >}}

When using the `{{ event_type }}` [dynamic variable]({{< relref "/docs/guide/authoringprs.md#dynamic-variables" >}}) for the following event types:

* `test-all-comment`
* `test-comment`
* `retest-all-comment`
* `retest-comment`
* `cancel-all-comment`
* `ok-to-test-comment`

The dynamic variable will return `pull_request` as the event type instead of the specific categorized GitOps command type. This is to handle backward compatibility with previous releases for users relying on this dynamic variable.

This currently only issues a warning in the repository matched namespace but will then be deprecated and changed to return the specific event type.
