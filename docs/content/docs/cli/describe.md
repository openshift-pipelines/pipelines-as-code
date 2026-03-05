---
title: "describe"
weight: 5
---

Use `tkn pac describe` to inspect a Repository CR and its associated PipelineRun history. This command is useful when you need to check run status, diagnose failures, or review recent activity for a repository.

## Usage

```shell
tkn pac describe [repo-name] [flags]
```

## Flags

* `--use-realtime`: Display timestamps as RFC3339 rather than relative time.
* `-t` / `--target-pipelinerun`: Show failures for a specific PipelineRun instead of the most recent one.

## Notes

When the most recent PipelineRun has failed, the command prints the last 10 lines of every task associated with that PipelineRun, highlighting `ERROR`, `FAILURE`, and other patterns. This helps you quickly identify what went wrong without switching to a dashboard.

On modern terminals (such as macOS Terminal, [iTerm2](https://iterm2.com/), [Windows Terminal](https://github.com/microsoft/terminal), GNOME Terminal, or kitty), the output links are clickable with Ctrl+click or Cmd+click. Clicking a link opens the console or dashboard URL for the associated PipelineRun. See your terminal documentation for details.
