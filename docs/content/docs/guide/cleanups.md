---
title: PipelineRuns Cleanup
weight: 8
---
# PipelineRuns Cleanups

There can be many PipelineRuns in a user namespace and Pipelines-as-Code has
the ability to keep only a certain number of PipelineRuns and clean up the old
ones.

When your PipelineRun has this annotation :

```yaml
pipelinesascode.tekton.dev/max-keep-runs: "maxNumber"
```

Pipelines-as-Code sees this and will start cleaning up right after one of the
PipelineRuns finishes successfully, keeping only the last `maxNumber` of
PipelineRuns.

It will skip the `Running` or `Pending` PipelineRuns but will not skip the
PipelineRuns with `Unknown` status.

{{< hint info >}}
The setting can also be configured globally for a cluster via the [pipelines-as-code ConfigMap]({{< relref "/docs/install/settings.md" >}})
{{< /hint >}}
