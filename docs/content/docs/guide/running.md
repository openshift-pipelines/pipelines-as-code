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
root of your repository from where the event comes from, this is unless you have
configured the [provenance from the default
branch](../repositorycrd/#pipelinerun-definition-provenance) on your Repository
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

- The author of the pull request is listed in the `OWNERS` file located in the main
  directory of the default branch on GitHub or your other service provider.

  The OWNERS file adheres to a specific format, similar to the Prow OWNERS
  file format (available at <https://www.kubernetes.dev/docs/guide/owners/>). We
  support simple OWNERS configuration including `approvers` and `reviewers` lists
  and they are treated equally in terms of permissions for executing a PipelineRun.
  If the OWNERS file includes `filters` instead of a simple OWNERS configuration,
  we only look for the everything matching `.*` filter and take the `approvers`
  and `reviewers` lists from there. All other filters (matching specific files or
  directories) are ignored.

  OWNERS_ALIASES is also supported and can be used for mapping of an alias name
  to a list of usernames.

  When you include contributors to the lists of `approvers` or `reviewers` in your
  OWNERS files, Pipelines-as-Code enables those contributors to execute a PipelineRun.

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

## Cancelling

### Cancelling in-progress PipelineRuns

{{< tech_preview "Cancelling in progress PipelineRuns" >}}

You can choose to cancel a PipelineRun that is currently in progress. This can
be done by adding the annotation `pipelinesascode.tekton.dev/cancel-in-progress:
"true"` in the PipelineRun definition.

This feature is effective only when the `PipelineRun` is in progress. If the
`PipelineRun` has already completed or been cancelled, the cancellation will
have no effect. (see the [max-keep-run annotation]({{< relref
"/docs/guide/cleanups.md" >}}) instead to clean old `PipelineRuns`.)

The cancellation only applies to `PipelineRuns` within the scope of the current
`PullRequest` or the targeted branch for Push events. For example, if two
`PullRequests` each have a `PipelineRun` with the same name and the
cancel-in-progress annotation, only the `PipelineRun` in the specific PullRequest
will be cancelled. This prevents interference between separate PullRequests.

The cancellation of the older `PipelineRuns` will be executed only after the
latest `PipelineRun` has been created and started successfully. This annotation
cannot guarantee that only one `PipelineRun` will be active at any given time.

Currently, `cancel-in-progress` cannot be used in conjunction with the [concurrency
limit]({{< relref "/docs/guide/repositorycrd.md#concurrency" >}}) setting.

### Cancelling a PipelineRun with a GitOps command

See [here]({{< relref "/docs/guide/gitops_commands.md#cancelling-a-pipelinerun" >}})

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
