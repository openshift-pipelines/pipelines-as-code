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
  - The author is a public member on the organization of the repository.
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

## PipelineRun Execution

The PipelineRun will always run in the namespace of the Repository CRD associated with the repo
that generated the event.

You can follow the execution of your pipeline with the
[tkn](https://github.com/tektoncd/cli) cli :

```console
tkn pr logs -n my-pipeline-ci -Lf
```

If you need to show another pipelinerun than the last one you
can use the `tkn pac` logs command :

```console
tkn pac logs -n my-pipeline-ci
```

If you have connected Pipelines as Code to the tekton dashboard or the
OpenShift console. Pipelines as Code will post a URL in the Checks tab for
GitHub apps to let you click on it and follow the pipeline execution directly
there.

## Restarting the PipelineRun

You can restart a PipelineRun without having to send a new commit to
your branch or pull_request.

### GitHub APPS

On GitHub if you are using the GitHub apps, you can go to the Checks tab and
click on the upper left button called "Re-Run" and Pipelines as Code will react
to the event and restart testing the PipelineRun.

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
