# Pipelines-as-Code

[![Latest Release](https://img.shields.io/github/v/release/openshift-pipelines/pipelines-as-code)](https://github.com/openshift-pipelines/pipelines-as-code/releases/latest)
[![Container Repository on GHCR](https://img.shields.io/badge/GHCR-image-87DCC0.svg?logo=GitHub)](https://github.com/openshift-pipelines/pipelines-as-code/pkgs/container/pipelines-as-code)
[![Go Report Card](https://goreportcard.com/badge/google/ko)](https://goreportcard.com/report/openshift-pipelines/pipelines-as-code)
[![E2E Tests](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/e2e.yaml/badge.svg)](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/e2e.yaml)
[![License](https://img.shields.io/github/license/openshift-pipelines/pipelines-as-code)](LICENSE)

Pipelines-as-Code is an opinionated CI/CD solution for Tekton and OpenShift Pipelines that allows you to define and manage your pipelines directly from your source code repository.

## Overview

Pipelines-as-Code brings the "Pipelines-as-Code" methodology to Tekton. It provides a simple and declarative way to define your pipelines in your Git repository and have them automatically executed on your Kubernetes cluster. It integrates seamlessly with Git providers like GitHub, GitLab, Bitbucket, and Gitea, and provides feedback directly on your pull requests and commits.

## How it Works

Pipelines-as-Code works by listening to events from your Git provider. When a new event occurs (e.g., a new pull request is opened), it looks for a `.tekton/` directory in your repository. If it finds one, it will look for Tekton pipeline definitions and execute them.

## Key Features

- **Git-based workflow**: Define your Tekton pipelines in your Git repository and have them automatically triggered on Git events like push, pull request, and comments.
- **Multi-provider support**: Works with GitHub (via GitHub App & Webhook), GitLab, Gitea, Bitbucket Data Center & Cloud via webhooks.
- **Annotation-driven workflows**: Target specific events, branches, or CEL expressions and gate untrusted PRs with `/ok-to-test` and `OWNERS`; see [Running the PipelineRun](https://pipelinesascode.com/docs/guide/running/).
- **ChatOps style control**: `/test`, `/retest`, `/cancel`, and branch or tag selectors let you rerun or stop PipelineRuns from PR comments or commit messages; see [GitOps Commands](https://pipelinesascode.com/docs/guide/gitops_commands/).
- **Feedback**: GitHub Checks capture per-task timing, log snippets, and optional error annotations while redacting secrets; see [PipelineRun status](https://pipelinesascode.com/docs/guide/statuses/).
- **Inline resolution**: The resolver bundles `.tekton/` resources, inlines remote tasks from Artifact Hub or Tekton Hub, and validates YAML before cluster submission; see [Resolver](https://pipelinesascode.com/docs/guide/resolver/).
- **CLI**: `tkn pac` bootstraps installs, manages Repository CRDs, inspects logs, and resolves runs locally; see the [CLI guide](https://pipelinesascode.com/docs/guide/cli/).
- **Automated housekeeping**: Keep namespaces tidy with the `pipelinesascode.tekton.dev/max-keep-runs` annotation or global settings, and automatically cancel running PipelineRuns when new commits are pushed to the same branch; see [PipelineRuns Cleanup](https://pipelinesascode.com/docs/guide/cleanups/) and [Cancel in progress](https://pipelinesascode.com/docs/guide/running/#cancelling-in-progress-pipelineruns).

## Quick Example

Here's a simple example of a Tekton pipeline triggered by pull requests:

```yaml
# .tekton/pull-request.yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pr-build
  annotations:
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
spec:
  pipelineSpec:
    tasks:
    - name: fetch-repository
      taskRef:
        name: git-clone
        resolver: hub
      workspaces:
      - name: output
        workspace: source
      params:
      - name: url
        value: "{{ repo_url }}"
      - name: revision
        value: "{{ revision }}"
    - name: run-tests
      runAfter: [fetch-repository]
      taskRef:
        name: golang-test
        resolver: hub
      workspaces:
      - name: source
        workspace: source
  workspaces:
  - name: source
    emptyDir: {}
```

This pipeline will automatically run on every pull request to the `main` branch, fetch the code, and run tests.

## Getting Started

### Installation

The easiest way to get started is by using the `tkn pac` CLI. You can install it with:

```shell
brew install openshift-pipelines/pipelines-as-code/tkn-pac
```

For other installation methods, see the [official documentation](https://pipelinesascode.com/docs/install/cli/).

### Creating your first repository

Once you have the `tkn-pac` CLI installed, you can set up your first repository with the `bootstrap` command. We have a full walk-through tutorial here:

<https://pipelinesascode.com/docs/install/getting-started/>

## Documentation

For more detailed information, please refer to the [official documentation](https://pipelinesascode.com).

The documentation for the development branch is available [here](https://nightly.pipelines-as-code.pages.dev/).

## Contributing

We welcome contributions! If you would like to help improve Pipelines-as-Code, please see our [contribution guide](https://pipelinesascode.com/docs/dev/).

## Community

- **Slack**: Join us on the TektonCD Slack in the [#pipelinesascode](https://tektoncd.slack.com/archives/C04URDDJ9MZ) channel. ([Join TektonCD Slack](https://github.com/tektoncd/community/blob/main/contact.md#slack))
- Use our Github Discussions for questions and community support: [GitHub Discussions](https://github.com/openshift-pipelines/pipelines-as-code/discussions)

## License

This project is licensed under the [Apache 2.0 License](LICENSE).
