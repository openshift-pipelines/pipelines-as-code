---
title: "resolve"
weight: 8
---

Use `tkn pac resolve` to process a PipelineRun locally as Pipelines-as-Code would on the server. This lets you test and debug your PipelineRun templates without creating a commit or triggering a webhook.

## Usage

```shell
tkn pac resolve -f <file-or-directory> [flags]
```

## Flags

* `-f`: Path to a PipelineRun YAML file or a directory containing YAML files. You can specify multiple `-f` flags.
* `-p`: Override a parameter (for example, `-p revision=main -p repo_name=othername`).
* `-o`: Write the resolved output to a file instead of stdout.
* `-B` / `--v1beta1`: Output the PipelineRun as v1beta1 (see [v1beta1 Compatibility](#v1beta1-compatibility)).
* `-t` / `--providerToken`: Provide a Git provider token on the command line.
* `--no-secret`: Skip secret generation entirely.

## Examples

To resolve a PipelineRun and apply it to your cluster:

```yaml
tkn pac resolve -f .tekton/pull-request.yaml -o /tmp/pull-request-resolved.yaml && kubectl create -f /tmp/pull-request-resolved.yaml
```

Combined with a local Kubernetes install (such as [CodeReady Containers](https://developers.redhat.com/products/codeready-containers/overview) or [Kubernetes Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)), you can see your run in action without generating a new commit.

To override parameters:

`tkn pac resolve -f .tekton/pr.yaml -p revision=main -p repo_name=othername`

## Notes

When you run this command from your source code repository, it detects parameters (such as `revision` or `branch_name`) from the local Git information.

Make sure the `git-clone` task (if used) can access the repository at the specified SHA. If you are testing your current source code, push it first before using `tkn pac resolve | kubectl create -`.

Unlike running on CI, you must explicitly specify the filenames or directories where your templates are located.

## v1beta1 Compatibility

On certain clusters, the conversion from v1beta1 to v1 in Tekton may not function correctly. This can cause errors when you apply the resolved PipelineRun on a cluster that does not have the bundle feature enabled. Use the `--v1beta1` flag (or `-B`) to output the PipelineRun as v1beta1 and work around this issue.

## Authentication

When the resolver detects a `{{
git_auth_secret }}` string inside your template, it prompts you to provide a Git provider token. If an existing secret in your namespace matches your repository URL, the command uses it automatically.

You can provide a token explicitly with the `-t` or `--providerToken` flag, or set the `PAC_PROVIDER_TOKEN` environment variable to avoid the prompt.

Use the `--no-secret` flag to skip secret generation entirely.

{{< callout type="info" >}}
The secret is not cleaned up after the run.
{{< /callout >}}
