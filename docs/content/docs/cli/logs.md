---
title: "logs"
weight: 7
---

Use `tkn pac logs` to stream the logs of a PipelineRun attached to a Repository CR. This command is useful when you want to review pipeline output directly from the terminal without opening a dashboard.

## Usage

```shell
tkn pac logs [repo-name] [flags]
```

## Flags

* `-w`: Open the console or dashboard URL for the log in your browser instead of streaming it to the terminal.

## Notes

If you do not specify a repository on the command line, the command prompts you to choose one, or auto-selects it if only one exists. If multiple PipelineRuns are attached to the Repository CR, you are prompted to choose one.

{{< callout type="info" >}}
The [`tkn`](https://github.com/tektoncd/cli) binary must be installed to show the logs.
{{< /callout >}}
