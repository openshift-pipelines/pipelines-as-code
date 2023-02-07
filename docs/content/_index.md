---
bookToc: false
title: An opinionated CI based on OpenShift Pipelines / Tekton
---
# Pipelines as Code

An opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as code is a project allowing you to define your CI/CD using
[Tekton](https://tekton.dev) PipelineRuns and Tasks in a file located in your
source control management (SCM) system, such as GitHub or Gitlab. This file is
then used to automatically create a pipeline for a Pull Request or a Push to a
branch.

By storing the pipeline definition in code, it becomes easier to version,
review, and collaborate on pipeline changes with code changes. Additionally, it
allows you to view the pipeline status and control its execution directly from
your SCM, rather than having to switch between multiple systems.

This approach enables automation, repeatability, collaboration, and change
tracking using a Git workflow.

## Features

{{< columns >}} <!-- begin columns block -->

- Pull-request status support: When iterating over a Pull Request. Statuses and
  Control is done on GitHub.

- GitHub Checks API support to set the status of a PipelineRun including rechecks

- GitHub Pull Request and Push event support

<--->

- Pull-request "*GitOps*" actions through comments with  `/retest`, `/test <pipeline-name>` and so on.

- Automatic Task resolution in Pipelines (local Tasks, Tekton Hub, and remote URLs)

- Efficient use of GitHub blobs and objects API for retrieving configurations

<--->

- Git events Filtering and support for separate pipelines for each event

- Gitlab, Bitbucket Server, Bitbucket Cloud and GitHub Webhook support.

- `tkn-pac` plug-in for Tekton CLI for managing pipelines-as-code repositories and bootstrapping.

{{< /columns >}}

## Getting Started

The easiest way to get started is to use the `tkn pac` CLI and its [bootstrap](/docs/guide/cli/#commands) command.

Start downloading and install the tkn-pac CLI following [these instructions](/docs/guide/cli#install) and
while connected to your cluster (for example using [kind](https://kind.sigs.k8s.io/) for testing) run the command :

```bash
-$ tkn pac bootstrap
```

and follow the questions to get Pipelines as Code installed on your cluster.
It will then help you create a GitHub Application to connect your repositories to Pipelines as Code.
If you are in a source code project, it will immediately ask you if you want to have a sample `PipelineRun` for `Pipelines as Code`

## Walkthrough video

This 10-minute video will guide you through the `tkn-pac bootstrap` flow :

{{< youtube cNOqPgpRXQY >}}

## Documentation

For more details on the different installation methods please follow [the
installation document](/docs/install/overview) detailing the Pipelines as Code
installation steps.

If you need to use `Pipelines as Code` and author `PipelineRuns` you can follow
the [usage guide](/docs/guide)
