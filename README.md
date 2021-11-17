**This documentation refers to the main developement branch, the documentation for the released version is [here](https://github.com/openshift-pipelines/pipelines-as-code/blob/0.5.0/README.md)**

# Pipelines as Code

[![Container Repository on Quay](https://quay.io/repository/openshift-pipeline/pipelines-as-code/status "Container Repository on Quay")](https://quay.io/repository/openshift-pipeline/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code) [![Go Report Card](https://goreportcard.com/badge/google/ko)](https://goreportcard.com/report/openshift-pipelines/pipelines-as-code)

Pipelines as Code -- An opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as Code let you use
the [Pipelines as Code flow]([https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code))
directly with OpenShift Pipelines.

The goal of Pipelines as Code is to let you define your
[Tekton](https://tekton.cd) templates inside your source code repository and have the pipeline run and report the status
of the execution when triggered by a Pull Request or a Push.

Pipelines as Code features:

- Pull-request status support: When iterating over a Pull Request, status and control is done on the platform.

- GitHub Checks API support to set the status of a PipelineRun including rechecks

- GitHub Pull Request and Commit event support

- Pull-request actions in comments such as `/retest`

- Git events filtering and support for separate pipelines for each event

- Automatic Task resolution in Pipelines (local Tasks, Tekton Hub and remote URLs)

- Efficient use of GitHub blobs and objects API for retrieving configurations

- ACL over a GitHub organization or via a Prow style [`OWNER`](https://www.kubernetes.dev/docs/guide/owners/) file.

- `tkn-pac` plugin for Tekton CLI for managing pipelines-as-code repositories and bootstrapping.

- Bitbucket Server, Bitbucket Cloud and Github Webhook support.

## Installation Guide

The easiest way to get started is to use the `tkn pac` CLI and its bootstrap command.

Download and install first the tkn-pac CLI following [these instructions](./docs/cli.md#install).

Connected to your cluster launch the command :

```bash
-$ tkn pac bootstrap
```
and follow the questions and installation methods which will install Pipelines as Code on cluster and help you create a Github Application.

feel free to look over the `--help` to see the different options on how to install for example on Github Enteprise.

This 10 minute video will guide you thought the `tkn-pac bootstrap` flow :

[![Getting started to Pipelines as Code](https://img.youtube.com/vi/ytm3brml8is/0.jpg)](https://www.youtube.com/watch?v=ytm3brml8is)

For more details on the different installation method please follow [this document](docs/install.md) for installing Pipelines as Code on OpenShift.

## Getting Started

The flow for using pipelines as code generally begins with admin installing the Pipelines-as-Code infrastructure,
creating a GitHub App and sharing the GitHub App url across the organization for app teams to enable the app on their
GitHub repositories.

In order to enable the GitHub App provided by admin on your Git repository as
documented [here](https://docs.github.com/en/developers/apps/managing-github-apps/installing-github-apps). Otherwise you
can go to the *Settings > Applications* and then click on *Configure* button near the GitHub App you had created. In
the **Repository access** section, select the repositories that you want to enable and have access to Pipelines-as-code.

Once you have enabled your GitHub App for your GitHub repository, you can use the `pac` Tekton CLI plugin to bootstrap
pipelines as code:

```
$ git clone https://github.com/siamaksade/pipeline-as-code-demo
$ cd pipeline-as-code-demo
$ tkn pac repository create

? Enter the namespace where the pipeline should run (default: pipelines-as-code):  demo
? Enter the Git repository url containing the pipelines (default: https://github.com/siamaksade/pipeline-as-code-demo):
? Enter the target GIT branch (default: main):
? Enter the Git event type for triggering the pipeline:  pull_request
! Namespace demo is not created yet
? Would you like me to create the namespace demo? Yes
✓ Repository pipeline-as-code-demo-pull-request has been created in demo namespace
? Would you like me to create a basic PipelineRun file into the file .tekton/pull_request.yaml ? True
✓ A basic template has been created in /Users/ssadeghi/Projects/pipelines/pac-demo/.tekton/pull_request.yaml, feel free to customize it.
ℹ You can test your pipeline manually with : tkn-pac resolve -f .tekton/pull_request.yaml | kubectl create -f-
ℹ Don't forget to install the GitHub application into your repo https://github.com/siamaksade/pipeline-as-code-demo
✓ and we are done! enjoy :)))

```

The above command would create a `Repository` CRD in your `demo` namespace which is used to determine where the
PipelineRuns for your GitHub repository should run. It also generates an example pipeline in the `.tekton` folder.
Commit and push the pipeline to your repo to start using pipelines as code.

Note that even if Github application is the preferred method, Pipeline As Code
supports Github Webhook and Bitbucket Server/Cloud as well, see the [INSTALL guide](docs/install.md) for
reference

## Usage Guide

The usage guide available [here](./docs/guide.md) offer a comprehenive documentatiuon on how to use and configure Pipeline As Code.

A walkthought video is available [here](https://www.youtube.com/watch?v=Uh1YhOGPOes).

## Videos/Blog Posts

- [OpenShift Developer Experience Office Hours: Pipeline as Code with OpenShift Pipelines](https://www.youtube.com/watch?v=PhqzGsJnFEI)
- [How to make a release pipeline with Pipelines as Code](https://blog.chmouel.com/2021/07/01/how-to-make-a-release-pipeline-with-pipelines-as-code)
