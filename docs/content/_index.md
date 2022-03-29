---
bookToc: false
---
# Pipelines as Code

An opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as Code let you use
the [Pipelines as Code flow](https://www.thoughtworks.com/radar/techniques/pipelines-as-code)
directly with OpenShift Pipelines.

The goal of Pipelines as Code is to let you define your
[Tekton](https://tekton.cd) templates inside your source code repository and have the pipeline run and report the status
of the execution when triggered by a Pull Request or a Push.

## Features

{{< columns >}} <!-- begin columns block -->
- Pull-request status support: When iterating over a Pull Request, status and control is done on the GitHub.

- GitHub Checks API support to set the status of a PipelineRun including rechecks

- GitHub Pull Request and Push event support

<--->

- Pull-request actions in comments such as `/retest`

- Automatic Task resolution in Pipelines (local Tasks, Tekton Hub and remote URLs)

- Efficient use of GitHub blobs and objects API for retrieving configurations

<--->

- Git events Filtering and support for separate pipelines for each event

- Gitlab, Bitbucket Server, Bitbucket Cloud and Github Webhook support.

- `tkn-pac` plugin for Tekton CLI for managing pipelines-as-code repositories and bootstrapping.

{{< /columns >}}

## Installation Guide

The easiest way to get started is to use the `tkn pac` CLI and its bootstrap command.

Download and install first the tkn-pac CLI following [these instructions](/docs/guide/cli#install) and
while Connected to your cluster launch the command :


```bash
-$ tkn pac bootstrap
```

and follow the questions and installation methods which will install Pipelines as Code on cluster and help you create a Github Application.

feel free to look over the `--help` to see the different options on how to install for example on Github Enterprise.

## Walkthrough video

This 10 minute video will guide you thought the `tkn-pac bootstrap` flow :

{{< youtube ytm3brml8is >}}

## Documentation

For more details on the different installation method please follow [the
installation document](/docs/install/overview) detailling the Pipelines as Code
installation steps.

If you need to use `Pipelines as Code` and author `PipelineRuns` you can follow
the [usage guide](/docs/guide)
