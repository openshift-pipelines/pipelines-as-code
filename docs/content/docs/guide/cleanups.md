---
title: PipelineRuns Cleanup
weight: 8
---
# PipelineRuns Cleanups

There can be a lot of PipelineRuns into an user namespace and Pipelines as Code
has the ability to only keep a number of PipelineRuns that matches an event.

For example if the PipelineRun has this annotation :

```yaml
pipelinesascode.tekton.dev/max-keep-runs: "maxNumber"
```

Pipelines as Code sees this and will start cleaning up right after it finishes a
successful execution keeping only the maxNumber of PipelineRuns.

It will skip the `Running` PipelineRuns but will not skip the PipelineRuns with
`Unknown` status.
