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
The cleanup limit can also be set globally for all repositories using the
[`default-max-keep-runs`]({{< relref "/docs/api/configmap#param-default-max-keep-runs" >}})
ConfigMap field. The
[`max-keep-run-upper-limit`]({{< relref "/docs/api/configmap#param-max-keep-run-upper-limit" >}})
field caps the maximum value a user may specify in the annotation.
{{< /callout >}}
