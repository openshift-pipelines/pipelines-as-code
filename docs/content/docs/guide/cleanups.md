---
title: PipelineRuns Cleanup
weight: 8
---
# PipelineRuns Cleanups

There can be many PipelineRuns into a user namespace and Pipelines-as-Code has
the ability to only keep a certain amount of PipelineRuns and cleaning the old
ones.

When your PipelineRun has this annotation :

```yaml
pipelinesascode.tekton.dev/max-keep-runs: "maxNumber"
```

Pipelines-as-Code sees this and will start cleaning up right after one of the
PipelineRun finishes to a successful execution keeping only the last `maxNumber` of
PipelineRuns.

It will skip the `Running` or `Pending` PipelineRuns but will not skip the
PipelineRuns with `Unknown` status.

{{< hint info >}}
The setting can be as well configured globally for a cluster via the [Pipelines-as-Code ConfigMap]({{< relref "/docs/install/settings.md" >}})
{{< /hint >}}
