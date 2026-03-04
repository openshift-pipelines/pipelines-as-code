---
title: "Flow Diagrams"
weight: 3
---

This page shows the event flow diagrams for Pipelines-as-Code.

## Diagram of a Pull/Merge Request Flow

[![PAC Diagram](/svg/diagram.svg)](/svg/diagram.svg)

*Detailed sequence from Git event through controller, resolution, and PipelineRun execution. Click to open full size.*

## High-level flow

```mermaid
sequenceDiagram
  participant Git
  participant Controller
  participant Cluster
  Git->>Controller: Webhook (push / PR)
  Controller->>Controller: Match Repository CR, fetch .tekton/
  Controller->>Cluster: Create PipelineRun(s)
  Cluster->>Git: Status (via Watcher)
```
