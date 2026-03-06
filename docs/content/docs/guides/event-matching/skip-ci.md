---
title: Skip CI Commands
weight: 4
---

Sometimes you want to push a commit without triggering any PipelineRuns -- for example, when you are updating documentation, fixing a typo, or working on an incomplete feature. Pipelines-as-Code lets you include a skip command in your commit message to bypass PipelineRun execution for that commit.

## Supported commands

Include any of the following commands anywhere in your commit message to skip PipelineRun execution:

* `[skip ci]` - Skip continuous integration
* `[ci skip]` - Alternative format for skipping CI
* `[skip tkn]` - Skip Tekton PipelineRuns
* `[tkn skip]` - Alternative format for skipping Tekton

{{< callout type="info" >}}
Skip commands are case-sensitive and must be in lowercase with brackets.
{{< /callout >}}

## Examples

```text
docs: update README with installation instructions [skip ci]
```

or

```text
WIP: refactor authentication module

This is still in progress and not ready for testing yet.

[ci skip]
```

## How skip commands work

When a commit message contains a skip command, Pipelines-as-Code behaves as follows:

1. **Pull requests**: Pipelines-as-Code does not create any PipelineRuns when the PR opens or updates and the HEAD commit contains a skip command. A neutral status check appears on the PR indicating that CI was skipped.
2. **Push events**: Pipelines-as-Code does not create any PipelineRuns when you push a commit with a skip command. A neutral status check appears on the commit.

{{< callout type="info" >}}
Pipelines-as-Code creates a neutral status check on your Git provider to indicate that it acknowledged the commit but intentionally skipped CI. This helps you distinguish between commits where CI was skipped and commits where CI has not yet run.
{{< /callout >}}

## Overriding skip commands with GitOps commands

{{< callout type="warning" >}}
GitOps commands override skip CI commands. Even if a commit contains `[skip ci]`, you can still manually trigger PipelineRuns by posting any of the comments listed below on the pull request.
{{< /callout >}}

* `/test` -- trigger all matching PipelineRuns
* `/test <pipelinerun-name>` -- trigger a specific PipelineRun
* `/retest` -- retrigger failed PipelineRuns
* `/retest <pipelinerun-name>` -- retrigger a specific PipelineRun
* `/ok-to-test` -- allow CI for external contributors
* `/custom-comment` -- trigger a PipelineRun that has an `on-comment` annotation

This lets you skip automatic CI execution while keeping the ability to trigger builds manually when needed.

## Example: skipping CI, then triggering manually

```bash
# Initial commit with skip command
git commit -m "docs: update contributing guide [skip ci]"
git push origin my-feature-branch
# No PipelineRuns are created automatically
# A neutral status check is displayed on the commit/PR

# Later, you can manually trigger CI by commenting on the PR:
# /test
# This will create PipelineRuns despite the [skip ci] command
```

## When to use skip commands

Skip commands are useful for:

* Documentation-only changes
* README updates
* Comment or formatting changes
* Work-in-progress commits
* Minor typo fixes
* Configuration file updates that don't affect code

## When NOT to use skip commands

Avoid skip commands for:

* Code changes that affect functionality
* Changes to CI/CD pipeline definitions
* Dependency updates
* Any changes that should be tested before merging
