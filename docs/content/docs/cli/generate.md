---
title: "generate"
weight: 6
---

Use `tkn pac generate` to scaffold a starter PipelineRun in your `.tekton/` directory. This is the quickest way to create a working PipelineRun template when you are setting up a new repository with Pipelines-as-Code.

## Usage

```shell
tkn pac generate
```

## How It Works

Run this command from your source code directory. It detects the current Git information and automatically populates relevant fields in the generated PipelineRun.

The command also performs basic language detection and adds extra tasks depending on what it finds. For example, if it detects a file named `setup.py` at the repository root, it adds the [pylint task](https://artifacthub.io/packages/tekton-task/tekton-catalog-tasks/pylint) to the generated PipelineRun.
