---
title: OpenShift Pipelines 1.15 PAC release
---

OpenShift Pipelines 1.15 presents several new enhancements to Pipelines-as-Code. Below are the key updates.

## Much improved GitOps Commands

These commands allow you to make quick comments on a pull request to restart a
PipelineRun through Pipelines-as-Code.

Commonly used commands include `/test` pipelinerun to rerun a specific
pipelinerun, or `/retest` to rerun all PipelineRuns.

### Trigger PipelineRuns Irrespective of Annotations

Previously, to re-trigger PipelineRuns, they had to match specific annotations
such as `pipelinesascode.tekton.dev/on-event` set to pull_request. Now, this
constraint is removed, allowing you to trigger any PipelineRuns with `/test`
regardless of their annotation status. This is particularly useful if you need
to run a pipelinerun selectively before merging a PR, without it automatically
consuming resources with each update.

### Modify Parameters Dynamically via GitOps Commands

When executing a PipelineRun, several default parameters are provided. The new
GitOps commands now support adding arguments in key=value format, allowing you
to modify these parameters in real-time. For example:

```console
/test pipelinerun revision=main
```

This command would test the PipelineRun on the main branch rather than the
specific commit of your Pull Request. You can redefine both standard and custom
parameters defined in your Repository CR.

### Custom GitOps Commands

With the release of OpenShift Pipelines 1.15, a new annotation
`pipelinesascode.tekton.dev/on-comment` has been introduced. This annotation
triggers a PipelineRun when a comment matches a specified regex, allowing you to
define custom GitOps commands that initiate PipelineRuns based on specific
comment patterns.

Additionally, a new standard parameter `{{ trigger_comment }}` is available,
capturing the entire comment that initiated the PipelineRun.

### Automatic Error Clearing Between Retest Attempts

Previously, unresolvable errors in Pipelines-as-Code remained until the PR was
updated with a new SHA. Now, such errors are automatically cleared with each
`/retest`, enhancing the reliability and user experience.

## Improved Error Reporting for YAML Issues

Improper YAML parsing within the `.tekton` directory previously caused an abrupt
termination with a generic error message. Now, the system validates the YAML
file beforehand, providing specific validation errors directly through the Git
provider interface to aid in troubleshooting.

## Global Repository settings support

A Repository CR can be configured with a variety of settings, which are then
automatically applied across all Repository CRs in a cluster.

An administrator can set up a default Repository CR where the controller is
installed, typically in the `openshift-pipelines` namespace, to drive the
default behaviours.

The settings from this default Repository CR are applied to all
other Repository CRs unless they are explicitly overridden.

For instance, if you wish to standardize the `git_provider` information and use a
common secret with a git token across all repositories in the cluster, you can
specify these settings in the global Repository CR. Consequently, these settings
will be inherited by all individual Repository CRs that uses a Git provider that
needs to reference a token and a URL.

## Prow OWNERS_ALIASES support

Pipelines-as-Code has been supporting the `OWNERS` file for a while, but we have
just added another feature from prow to support the `OWNERS_ALIASES` file.

This file is used to define aliases for the `OWNERS` file, so you can define a
group of people that are responsible for a specific part of the codebase.
