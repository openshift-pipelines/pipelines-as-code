---
title: Running the PipelineRun
weight: 4
---
# Running the PipelineRun

Pipelines as Code will run any PipelineRuns committed to the default branch of the repo
when the specified events occur on the repo.
For example, if a PipelineRun on the default branch has the annotation
`pipelinesascode.tekton.dev/on-event: "[pull_request]"`, it will run whenever a pull request event occurs.

Pipelines as Code will also run any PipelineRuns from a branch in a pull request (or merge request in Gitlab).
For example, if you're testing out a new PipelineRun, you can create a pull request
with that PipelineRun, and it will run if the following conditions are met:

- The pull request author's PipelineRun will be run if:

  - The author is the owner of the repository.
  - The author is a collaborator on the repository.
  - The author is a member (public or private) on the repository's organization.
  - The author has permissions to push to branches inside the repository.
  - The pull request author is inside an OWNER file located in the
  repository root on the main branch (the main branch as defined in the GitHub
  configuration for the repo) and added to either `approvers` or `reviewers`
  sections. For example, if the approvers section looks like this:

```yaml
approvers:
  - approved
```

then the user `approved` will be allowed.

If the pull request author does not meet these requirements,
another user that does meet these requirements can comment `/ok-to-test` on the pull request
to run the PipelineRun.

{{< hint info >}}
If you are using the GitHub Apps and have installed it on an Organization,
Pipelines as Code will only be initiated when a Repo CR matching one of the
repositories in a URL is detected on a repository belonging to the organization
where the GitHub App has been installed.
Otherwise, Pipelines as Code will not be triggered.
{{< /hint >}}

## PipelineRun Execution

The PipelineRun will always run in the namespace of the Repository CRD associated with the repo
that generated the event.

You can follow the execution of your PipelineRun with the [tkn pac](../cli/#install) cli :

```console
tkn pac logs -n my-pipeline-ci -L
```

If you need to show another pipelinerun than the last one you can use the `tkn
pac` logs command and it will ask you to select a PipelineRun attached to the
repository :

```console
tkn pac logs -n my-pipeline-ci
```

If you have set-up Pipelines as Code with the [Tekton Dashboard](https://github.com/tektoncd/dashboard/)
or on OpenShift using the Openshift Console.
Pipelines as Code will post a URL in the Checks tab for GitHub apps to let you
click on it and follow the pipeline execution directly there.

## Restarting the PipelineRun

You can restart a PipelineRun without having to send a new commit to
your branch or pull_request.

### GitHub APPS

If you are using the GitHub apps method, you have the option to access the "Checks"
tab where you can find an upper right button labeled "Re-Run". By clicking on
this button, you can trigger Pipelines as Code to respond and recommence
testing the PipelineRun.

This feature enables you to either rerun a particular pipeline or execute the
entire suite of checks once again.

![github apps rerun check](/images/github-apps-rerun-checks.png)

### Gitops command on pull or merge request

If you are targetting a pull or merge request you can use `GitOps` comment
inside your pull request, to restart all or specific Pipelines.

For example you want to restart all your pipeline you can add a comment starting
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

Example:

```text
roses are red, violets are blue. why to run the pipeline when the infra is down.

/cancel <pipelinerun-name>
```

On GitHub App the status of the Pipeline will be set to `cancelled`.

![pipelinerun canceled](/images/pr-cancel.png)
