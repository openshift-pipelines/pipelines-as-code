---
title: PipelineRuns Cleanup
weight: 1
---
This page explains how to automatically remove old PipelineRuns so that completed runs do not accumulate and consume cluster resources. Use this when you want to retain only a fixed number of recent runs per PipelineRun definition.

## Configuring cleanup

To limit the number of retained runs, add the following annotation to your PipelineRun:

```yaml
pipelinesascode.tekton.dev/max-keep-runs: "maxNumber"
```

After a PipelineRun finishes successfully, Pipelines-as-Code detects this annotation and removes older PipelineRuns, keeping only the last `maxNumber` runs.

Pipelines-as-Code skips any PipelineRun in a `Running` or `Pending` state during cleanup. However, it does not skip PipelineRuns with an `Unknown` status.

{{< callout type="info" >}}
The setting can also be configured globally for a cluster via the [pipelines-as-code ConfigMap]({{< relref "/docs/operations/settings" >}})
{{< /callout >}}
