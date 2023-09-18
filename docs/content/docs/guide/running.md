---
title: Running the PipelineRun
weight: 4
---

# Running the PipelineRun

Pipelines-as-Code (PAC) can be used to run pipelines on events such as pushes
or pull requests. When an event occurs, PAC will try to match it to any
PipelineRuns located in the `.tekton` directory of your repository
that are annotated with the appropriate event type.

{{< hint info >}}
The PipelineRuns definitions are fetched from the `.tekton` directory at the
root of you repository from where the event come from, this is unless you have
configured the [provenance from the default
branch](../repositorycrd/#pipelinerun-definition-provenance) on you Repository
CR.
{{< /hint >}}

For example, if a PipelineRun has this annotation:

```yaml
pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

it will be matched when a pull request is created and run on the cluster, as
long as the submitter is allowed to run it.

The rules for determining whether a submitter is allowed to run a PipelineRun
on CI are as follows. Any of the following conditions will allow a submitter to
run a PipelineRun on CI:

- The author of the pull request is the owner of the repository.
- The author of the pull request is a collaborator on the repository.
- The author of the pull request is a public or private member of the organization that
  owns the repository.
- The author of the pull request has permissions to push to branches inside the
  repository.

- The author who initiated the pull request is identified in an `OWNERS` file
  found in the main directory of the branch that is set as the default branch
  on GitHub or your other service provider.

  The OWNERS file need adheres to a specific format, similar to the Prow OWNERS
  file format (available at <https://www.kubernetes.dev/docs/guide/owners/>),
  with the exception that we do not yet support the use of `OWNERS_ALIASES`.

  When you include contributors in the `approvers` or `reviewers` sections,
  Pipelines-as-Code enables those contributors to execute a PipelineRun listed
  in the OWNERS file.

  For instance, if the `approvers` section of your OWNERS file in the main or
  master branch of your repository appears as follows:

  ```yaml
  approvers:
    - approved
  ```

  then the user with the username "approved" will be granted permission.

If the pull request author does not have the necessary permissions to run a
PipelineRun, another user who does have the necessary permissions can comment
`/ok-to-test` on the pull request to run the PipelineRun.

{{< hint info >}}
If you are using the GitHub Apps and have installed it on an organization,
Pipelines-as-Code will only be triggered if it detects a Repo CR that matches
one of the repositories in a URL on a repository that belongs to the
organization where the GitHub App has been installed. Otherwise, Pipelines as
Code will not be triggered.
{{< /hint >}}

## PipelineRun Execution

The PipelineRun will always run in the namespace of the Repository CRD associated with the repo
that generated the event.

You can monitor the execution using the command line with the [tkn
pac](../cli/#install) CLI :

```console
tkn pac logs -n my-pipeline-ci -L
```

If you need to show another pipelinerun than the last one you can use the `tkn
pac` logs command and it will ask you to select a PipelineRun attached to the
repository :

```console
tkn pac logs -n my-pipeline-ci
```

If you have set-up Pipelines-as-Code with the [Tekton Dashboard](https://github.com/tektoncd/dashboard/)
or on OpenShift using the OpenShift Console.
Pipelines-as-Code will post a URL in the Checks tab for GitHub apps to let you
click on it and follow the pipeline execution directly there.

## Restarting the PipelineRun

You can restart a PipelineRun without having to send a new commit to
your branch or pull_request.

### GitHub APPS

If you are using the GitHub apps method, you have the option to access the "Checks"
tab where you can find an upper right button labeled "Re-Run". By clicking on
this button, you can trigger Pipelines-as-Code to respond and recommence
testing the PipelineRun.

This feature enables you to either rerun a particular pipeline or execute the
entire suite of checks once again.

![github apps rerun check](/images/github-apps-rerun-checks.png)

### GitOps command on pull or merge request

If you are targeting a push, pull or merge request you can use `GitOps` comment
inside your pull request, to restart all or specific Pipelines.

For example, you want to restart all your pipeline you can add a comment starting
with `/retest` and all PipelineRun attached to that pull or merge request will be
restarted :

Example :

```text
Thanks for contributing, This is a much needed bugfix, and we love it ❤️ The
failure is not with your PR but seems to be an infra issue.

/retest
```

If you have multiple `PipelineRun` and you want to target a specific
`PipelineRun` you can use the `/test` comment, example:

```text
roses are red, violets are blue. pipeline are bound to flake by design.

/test <pipelinerun-name>
```

### GitOps command on push request

To trigger GitOps commands in response to a push request, you can include `GitOps`
comments within your commit messages. These comments can be used to restart
either all pipelines or specific ones. Here's how it works:

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

To add `GitOps` comments to a push request, follow these steps:

1. Go to your repository.
2. Click on the **Commits** section.
3. Choose one of the individual **Commit**.
4. Click on the line number where you want to add a `GitOps` comment, as shown in the image below:

![GitOps Commits For Comments](/images/gitops-comments-on-commit.png)

Please note that this feature is supported for the GitHub provider only.

## Cancelling the PipelineRun

You can cancel a running PipelineRun by commenting on the PullRequest.

For example if you want to cancel all your PipelinerRuns you can add a comment starting
with `/cancel` and all PipelineRun attached to that pull or merge request will be cancelled.

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
