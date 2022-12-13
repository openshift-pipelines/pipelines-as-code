---
title: PipelineRun status
weight: 6
---
# Status

## GitHub apps

When the pipeline finishes, the status will be added in the GitHub Check tabs
with a short recap of how long each task of your pipeline took and the output of
`tkn pr describe`.

## Log error snippet

When we detect an error in one of the task of the Pipeline we will show a small
snippet of the last 3 lines in the task breakdown.

This will only show the output of the first failed task (due of the
limitation of the API not allowing to have many characters).

Pipelines as Code try to avoid leaking secrets by looking into the PipelineRun
and replace the secrets values with hidden characters.
We do this by fetching every secrets on environment variable attached to any
tasks and steps, check if there is any match of those values in the snippet and
*blindly* replace them with a `*****` placeholder.

This doesn't support hiding secrets coming from workspaces and
[envFrom](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#envfromsource-v1-core)
source.

![log snippet](/images/snippet-failure-message.png)

### Annotations (alpha feature)

If you set `error-detection-from-container-logs` to `true` in the
`pipeline-as-code` [config map](/docs/install/settings.md), pipelines-as-code
will try to detect the errors from the container logs and add them as
annotations on the Pull Request where the error occurred.

We currently support only the simple case  where the error looks like `makefile` or `grep` output of this format:

```console
filename:line:column: error message
```

tools like `golangci-lint`, `pylint`, `yamllint` and many others are able to
output errors in this format.

As an example you can see how `Pipelines as Code` [pull_request.yaml](https://github.com/openshift-pipelines/pipelines-as-code/blob/7c9b16409a1a6c93e9480758f069f881e5a50f05/.tekton/pull-request.yaml#L70) will pass the right arguments to the binary to output in that format.

You can customize the regexp used to detect the errors with the
`error-detection-simple-regexp` setting. The regexp used [named
groups](https://www.regular-expressions.info/named.html) to give flexibility on
how to specify the matching. The groups needed to match is `filename`, `line` and `error`
(`column` is not used) see the default regexp in the config map.

By default Pipelines as code will only look for the last 50 lines of the container
logs. You can increase this value in the `error-detection-max-number-of-lines`
setting or set `-1` for an unlimited number of lines. This may increase the memory
usage of the watcher.

![annotations](/images/github-annotation-error-failure-detection.png)

## Webhook

On webhook when the event is a pull request it will be added as a comment of the
pull or merge request.

For push event there is other method to get the status of the pipeline.

## Failures

If a namespace has been matched to a Repository, Pipelines As Code will emit its log messages in the kubernetes events inside the `Repository`'s namespace.

## Repository CRD

Status of your pipeline execution is stored inside the Repo CustomResource.

```console
% kubectl get repo -n pipelines-as-code-ci
NAME                  URL                                                        NAMESPACE             SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipelines-as-code-ci   https://github.com/openshift-pipelines/pipelines-as-code   pipelines-as-code-ci   True        Succeeded   59m         56m
```

The last 5 status are stored inside the Repository CR.

Using [tkn pac](../cli/)  describe, you can easily all the statuses of the Runs attached to your repository and its metadatas.

## Notifications

Notifications are not managed by Pipelines as Code.

If you need to have some other type of notification you can use
the [finally feature of tekton pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#adding-finally-to-the-pipeline)
.

Here is an example task to send a Slack message on failures or successes:
<https://github.com/chmouel/tekton-slack-task-status>

As a complete example you can have a look into the push pipeline and how Pipelines as Code uses it
to send a slack message if there is any failures while generating the artifacts on every push:
[Pipelines as code push pipeline](https://github.com/openshift-pipelines/pipelines-as-code/blob/7b41cc3f769af40a84b7ead41c6f037637e95070/.tekton/push.yaml#L116)
