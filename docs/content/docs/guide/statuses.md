---
title: PipelineRun status
weight: 6
---
# Status

## GitHub apps

When the pipeline finishes, the status will be added in the GitHub Check tabs
with a short recap of how long each task of your pipeline took and the output of
`tkn pr describe`.

### Annotations (alpha feature)

If you set `error-detection-from-container-logs` to `true` in the
`pipeline-as-code` [configmap](/docs/install/settings.md), pipelines-as-code
will try to detect the errors from the container logs and add them as
annotations on the Pull Request where the error occured.

We currently support only the simple case  where the error looks like `makefile` or `grep` output of this format:

```console
filename:line:column: error message
```

tools like `golangci-lint`, `pylint`, `yamllint` and many others are able to output errors in this format.

You can customize the regexp used to detect the errors with the
`error-detection-simple-regexp` setting. The regexp used [named
groups](https://www.regular-expressions.info/named.html) to give flexibility on
how to specify the matching. The groups needed to match is `filename`, `line` and `error`
(`column` is not used) see the default regexp in the configmap.

By default pipelines as code will look for the last 50 lines of the container
logs. You can increase this value in the `error-detection-max-number-of-lines`
setting or set `-1` for unlimited number of lines. This may increase the memory
usage of the watcher.

![annotations](/images/github-annotation-error-failure-detection.png)

## Webhook

On webhook when the event is a pull request it will be added as a comment of the
pull or merge request.

For push event there is other method to get the status of the pipeline.

## Failures

If a namespace has been matched to a Repository, Pipelines As Code will emit its log messages in the kubernetes events inside the `Repository`'s namespace.

## Repository CRD

Status of your pipeline execution is stored inside the Repo CustomResource :

```console
% kubectl get repo -n pipelines-as-code-ci
NAME                  URL                                                        NAMESPACE             SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipelines-as-code-ci   https://github.com/openshift-pipelines/pipelines-as-code   pipelines-as-code-ci   True        Succeeded   59m         56m
```

The last 5 status are stored inside the CustomResource and can be accessed
directly like this :

```console
% kubectl get repo -n pipelines-as-code-ci -o json|jq .items[].pipelinerun_status
[
  {
    "completionTime": "2021-05-05T11:00:05Z",
    "conditions": [
      {
        "lastTransitionTime": "2021-05-05T11:00:05Z",
        "message": "Tasks Completed: 3 (Failed: 0, Cancelled 0), Skipped: 0",
        "reason": "Succeeded",
        "status": "True",
        "type": "Succeeded"
      }
    ],
    "pipelineRunName": "pipelines-as-code-test-run-7tr84",
    "startTime": "2021-05-05T10:53:43Z"
  },
  {
    "completionTime": "2021-05-05T11:20:18Z",
    "conditions": [
      {
        "lastTransitionTime": "2021-05-05T11:20:18Z",
        "message": "Tasks Completed: 3 (Failed: 0, Cancelled 0), Skipped: 0",
        "reason": "Succeeded",
        "status": "True",
        "type": "Succeeded"
      }
    ],
    "pipelineRunName": "pipelines-as-code-test-run-2fhhg",
    "startTime": "2021-05-05T11:11:20Z"
  },
  […]
```

## Notifications

Notifications are not handled by Pipelines as Code, the only place where we
notify a status in an interface is when we do a Pull Request on for example the
GitHub checks interface to show the results of the pull request.

If you need some other type of notification you can use
the [finally feature of tekton pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#adding-finally-to-the-pipeline)
.

Here is an example task to send a Slack message on failures (or success if you
like) :

<https://github.com/chmouel/tekton-slack-task-status>

The push pipeline of Pipelines as Code use this task, you can see the example
here :

[.tekton/push.yaml](https://github.com/openshift-pipelines/pipelines-as-code/blob/7b41cc3f769af40a84b7ead41c6f037637e95070/.tekton/push.yaml#L116)
