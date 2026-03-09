---
title: Path-Based Matching
weight: 1
---

Path-based matching lets you run PipelineRuns only when specific files change in a pull request or push -- for example, running documentation tests only when files under `docs/` change, or skipping CI entirely when only markdown files are modified. This page covers the `on-path-change` and `on-path-change-ignore` annotations.

{{< tech_preview "Matching a PipelineRun to specific path changes via annotation" >}}

## Triggering on path changes

To trigger a PipelineRun only when certain files change, use the annotation `pipelinesascode.tekton.dev/on-path-change`. You can specify multiple glob patterns separated by commas. Pipelines-as-Code triggers the PipelineRun when the first glob matches a changed file. To match a file or path that contains a literal comma, escape it with the `&#44;` HTML entity.

You still need to specify the event type and target branch alongside this annotation. If your PipelineRun also has a [CEL expression]({{< relref "/docs/guides/event-matching/cel-expressions#matching-by-path-change" >}}), Pipelines-as-Code ignores the `on-path-change` annotation.

Example:

```yaml
metadata:
  name: pipeline-docs-and-manual
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[docs/**.md, manual/**.rst]"
```

Pipelines-as-Code triggers the PipelineRun `pipeline-docs-and-manual` when a `pull_request` event targets the `main` branch and the changeset includes `.md` files in the `docs` directory (and its subdirectories) or `.rst` files in the `manual` directory.

{{< callout type="info" >}}
These patterns are [glob](https://en.wikipedia.org/wiki/Glob_(programming)) patterns, not regex. See the [glob library examples](https://github.com/gobwas/glob?tab=readme-ov-file#example) for the full syntax.

You can test your glob patterns locally with the `tkn pac` CLI [globbing command]({{< relref "/docs/cli/" >}}):

```bash
tkn pac info globbing "[PATTERN]"
```

matches files against `[PATTERN]` in the current directory.

{{< /callout >}}

## Ignoring specific path changes

{{< tech_preview "Matching a PipelineRun to ignore specific path changes via annotation" >}}

The inverse of `on-path-change` is `pipelinesascode.tekton.dev/on-path-change-ignore`. Use it to trigger a PipelineRun only when changes fall outside the specified paths. This is useful when you want to skip CI for documentation-only or config-only changes.

You still need to specify the event type and target branch. If your PipelineRun also has a [CEL expression]({{< relref "/docs/guides/event-matching/cel-expressions#matching-by-path-change" >}}), Pipelines-as-Code ignores the `on-path-change-ignore` annotation.

The following PipelineRun runs only when changes occur outside the `docs` directory:

```yaml
metadata:
  name: pipeline-not-on-docs-change
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[docs/***]"
```

You can combine `on-path-change` and `on-path-change-ignore` to include some paths while excluding others:

```yaml
metadata:
  name: pipeline-docs-not-generated
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[docs/***]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[docs/generated/***]"
```

Pipelines-as-Code triggers this PipelineRun when there are changes in the `docs` directory but not in the `docs/generated` directory.

### Precedence rules

The `on-path-change-ignore` annotation always takes precedence over `on-path-change`. For example, with these annotations:

```yaml
metadata:
  name: pipelinerun-go-only-no-markdown-or-yaml
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-path-change: "[***.go]"
    pipelinesascode.tekton.dev/on-path-change-ignore: "[***.md, ***.yaml]"
```

if a pull request changes the files `.tekton/pipelinerun.yaml`, `README.md`, and `main.go`, Pipelines-as-Code does not trigger the PipelineRun because the `on-path-change-ignore` annotation excludes the `***.md` and `***.yaml` files.
