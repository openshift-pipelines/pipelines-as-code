---
title: Metrics
weight: 16
---

# Metrics Overview

The metrics for pipelines-as-code can be accessed through the `pipelines-as-code-watcher` service on port `9090`.

pipelines-as-code supports various exporters, such as Prometheus, Google Stackdriver, and more.
You can configure these exporters by referring to the [observability configuration](../config/config-observability.yaml).

|  Name | Type    | Description                                         |
| ---------- |---------|-----------------------------------------------------|
| `pipelines_as_code_pipelinerun_count` | Counter | Number of pipelineruns created by pipelines-as-code |
