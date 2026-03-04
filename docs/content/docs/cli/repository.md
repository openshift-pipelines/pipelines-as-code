---
title: "list"
weight: 4
---

Use `tkn pac list` to get an overview of all Repository CRs and their current PipelineRun status. This is helpful when you want to quickly check which repositories Pipelines-as-Code is managing and whether their latest runs succeeded.

## Usage

```shell
tkn pac list [flags]
```

## Flags

* `-A` / `--all-namespaces`: List all Repository CRs across the cluster (requires appropriate permissions).
* `-l` / `--selectors`: Filter repositories by labels.
* `--use-realtime`: Display timestamps as RFC3339 rather than relative time.

## Notes

`tkn pac list` displays every Repository CR and shows the last or current status (if running) of the PipelineRun associated with each one.

On modern terminals (such as macOS Terminal, [iTerm2](https://iterm2.com/), [Windows Terminal](https://github.com/microsoft/terminal), GNOME Terminal, or kitty), the output links are clickable with Ctrl+click or Cmd+click. Clicking a link opens the console or dashboard URL for the associated PipelineRun. See your terminal documentation for details.
