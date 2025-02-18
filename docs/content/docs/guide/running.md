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

it will be automatically triggered and executed when a user with appropriate permissions submits a Pull Request. See ACL Permissions for triggering PipelineRuns below.

When using GitHub as a provider, Pipelines-as-Code runs on draft Pull Requests by default. However, you can prevent pipelines from triggering on draft Pull Requests by using the following annotation:

```yaml
pipelinesascode.tekton.dev/on-cel-expression: event == "pull_request" && !body.pull_request.draft
```

With this configuration, your pipeline will only be triggered when the Pull Request is converted to "Ready for Review." For additional examples, see [Advanced event matching using CEL](https://pipelinesascode.com/docs/guide/matchingevents/#advanced-event-matching-using-cel).

And if you are using the GitHub provider with GitHub Apps and have installed it
on an organization, Pipelines-as-Code will only be triggered if it detects a
Repo CR that matches one of the repositories in a URL on a repository that
belongs to the organization where the GitHub App has been installed. Otherwise,
Pipelines-as-Code will not be triggered.

## ACL Permissions for triggering PipelineRuns

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
(see below for the OWNERS file format).

If an unauthorized user attempts to trigger a PipelineRun through the creation
of a Pull Request or by any other means, Pipelines-as-Code will block the
execution and post a `'Pending'` status check. This check will inform the user
that they lack the necessary permissions. Only authorized users can initiate the
PipelineRun by commenting `/ok-to-test` on the pull request.

GitHub bot users, identified through the GitHub API, are generally exempt from
the `Pending` status check that would otherwise block a pull request. This
means the status check is silently ignored for bots unless they have been
explicitly authorized (using [OWNERS](#owners-file) file,
[Policy]({{< relref "/docs/guide/policy" >}}) or other means).

## OWNERS file

The `OWNERS` file follows a specific format similar to the Prow `OWNERS` file
format (detailed at <https://www.kubernetes.dev/docs/guide/owners/>). We
support a basic `OWNERS` configuration with `approvers` and `reviewers` lists,
both of which have equal permissions for executing a `PipelineRun`.

If the `OWNERS` file uses `filters` instead of a simple configuration, we only
consider the `.*` filter and extract the `approvers` and `reviewers` lists from
it. Any other filters targeting specific files or directories are ignored.

Additionally, `OWNERS_ALIASES` is supported and allows mapping alias names to a
lists of usernames.

Including contributors in the `approvers` or `reviewers` lists within your
`OWNERS` file grants them the ability to execute a `PipelineRun` via
Pipelines-as-Code.

For example, if your repository’s `main` or `master` branch contains the
following `approvers` section:

```yaml
approvers:
  - approved
```

The user with the username `"approved"` will have the necessary
permissions.

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

## Errors When Parsing PipelineRun YAML

When Pipelines-As-Code encounters an issue with the YAML formatting in the
repository, it will log the error in the user namespace events log and
the Pipelines-as-Code controller log.

Despite the error, Pipelines-As-Code will continue to run other correctly parsed
and matched PipelineRuns.

{{< support_matrix github_app="true" github_webhook="true" gitea="true" gitlab="true" bitbucket_cloud="false" bitbucket_server="false" >}}

When an event is triggered from a Pull Request, a new comment will be created on
the Pull Request detailing the error.

Subsequent iterations on the Pull Request will update the comment with any new
errors.

If no new errors are detected, the comment will remain and will not be deleted.

Here is an example of a YAML error being reported as a comment to a Pull Request:

![report yaml error as comments](/images/report-error-comment-on-bad-yaml.png)

## Cancelling

### Cancelling in-progress PipelineRuns

{{< tech_preview "Cancelling in progress PipelineRuns" >}}
{{< support_matrix github_app="true" github_webhook="true" gitea="true" gitlab="true" bitbucket_cloud="true" bitbucket_datacenter="false" >}}

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

Older `PipelineRuns` are canceled only after the latest `PipelineRun` is
successfully created and started. This annotation does not guarantee that only
one `PipelineRun` will be active at a time.

If a `PipelineRun` is in progress and the Pull Request is closed or declined,
the `PipelineRun` will be canceled.

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
