# Pipelines as Code

[![Container Repository on GHC](https://img.shields.io/badge/GHCR-image-87DCC0.svg?logo=GitHub)](https://github.com/openshift-pipelines/pipelines-as-code/pkgs/container/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code) [![Go Report Card](https://goreportcard.com/badge/google/ko)](https://goreportcard.com/report/openshift-pipelines/pipelines-as-code) [![E2E Tests](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/kind-e2e-tests.yaml/badge.svg)](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/kind-e2e-tests.yaml)

Pipelines as Code -- An opinionated CI based on OpenShift Pipelines / Tekton.

Full documentation for the stable version is available from <https://pipelinesascode.com>
Documentation for the developement branch is available [here](https://nightly.pipelines-as-code.pages.dev/)

## Introduction

Pipelines as Code let you use
the [Pipelines as Code flow]([https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code)) directly with Tekton and OpenShift Pipelines.

The goal of Pipelines as Code is to let you define your
[Tekton](https://tekton.dev) templates inside your source code repository and have the pipeline run and report the status
of the execution when triggered by a Pull Request or a Push.

Pipelines as Code features:

- Pull-request status support: When iterating over a Pull Request, status and control is done on the platform.

- GitHub Checks API support to set the status of a PipelineRun including rechecks

- GitHub Pull Request and Commit event support

- Pull-request actions in comments such as `/retest`

- Git events filtering and support for separate pipelines for each event

- Automatic Task resolution in Pipelines (local Tasks, Tekton Hub and remote URLs)

- Efficient use of GitHub blobs and objects API for retrieving configurations

- ACL over a GitHub organization or with a Prow style [`OWNER`](https://www.kubernetes.dev/docs/guide/owners/) file.

- `tkn-pac` plug-in for Tekton CLI for managing pipelines-as-code repositories and bootstrapping.

- Gitlab, Bitbucket Server, Bitbucket Cloud and GitHub through Webhook support.

## Installation Guide

The easiest way to get started is to use the `tkn pac` CLI and its bootstrap command.

Download and install first the tkn-pac CLI following [these instructions](/docs/content/docs/guide/cli.md#install).

Connected to your cluster execute the command :

```bash
-$ tkn pac bootstrap
```

and follow the questions and installation methods which will install Pipelines as Code on cluster and help you create a GitHub Application.

feel free to look over the `--help` to see the different options on how to install for example on GitHub Enterprise.

This 10 minutes video will guide you thought the `tkn-pac bootstrap` flow :

[![Getting started to Pipelines as Code](https://img.youtube.com/vi/ytm3brml8is/0.jpg)](https://www.youtube.com/watch?v=ytm3brml8is)

For more details on the different installation method please follow [this document](docs/install.md) for installing Pipelines as Code on OpenShift.

## Getting Started

The flow for using pipelines as code generally begins with admin installing the Pipelines-as-Code infrastructure,
creating a GitHub App and sharing the GitHub App URL across the organization for app teams to enable the app on their
GitHub repositories.

To enable the GitHub App provided by admin on your Git repository as documented
[here](https://docs.github.com/en/developers/apps/managing-github-apps/installing-github-apps).
Otherwise, you can go to the *Settings > Applications* and then click on
*Configure* button near the GitHub App you had created. In the **Repository
access** section, select the repositories that you want to enable and have
access to Pipelines-as-code.

Once you have enabled your GitHub App for your GitHub repository, you can use the `pac` Tekton CLI plug-in to bootstrap
pipelines as code:

```bash
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

Note that even if installing with GitHub application is the preferred installation method, Pipeline As Code
supports other methods :

- GitHub direct Webhook
- Gitlab public and private instances.
- Bitbucket Cloud
- Bitbucket Server

see the [INSTALL guide](https://pipelinesascode.com/docs/install/) more details on each install method.

## Usage Guide

The usage guide available [here](https://pipelinesascode.com/docs/guide/) offer a comprehensive documentation on how to use and configure Pipeline As Code.

A walkthrough video is available [here](https://www.youtube.com/watch?v=Uh1YhOGPOes).

## Developer Guide

If you want to help and contribute to the `pipelines-as-code` project, you can
see the documentation here to get started: <https://pipelinesascode.com/dev/>
(and thank you).

## Videos/Blog Posts

- [OpenShift Developer Experience Office Hours: Pipeline as Code with OpenShift Pipelines](https://www.youtube.com/watch?v=PhqzGsJnFEI)
- [How to make a release pipeline with Pipelines as Code](https://blog.chmouel.com/2021/07/01/how-to-make-a-release-pipeline-with-pipelines-as-code)
- Main branch documentation - <https://main.pipelines-as-code.pages.dev/>
