---
title: "Using PaC's `git_auth_secret` to Avoid Rate Limiting"
date: 2025-07-30
---

## Using PaC's `git_auth_secret` to Avoid Rate Limiting

When Tekton pipelines fetch resources like `Tasks` or `Pipelines` from a Git repository using the **`git` resolver**, frequent unauthenticated requests can lead to **rate limiting** from your Git provider. For private repositories, this fetching would fail entirely without authentication.

Pipelines-as-Code (PaC) solves this elegantly by automatically generating a temporary, scoped authentication token for each `PipelineRun`. This token is stored in a Kubernetes `Secret`, and its name is made available to your `PipelineRun` through the built-in `{{ git_auth_secret }}` variable.

This guide shows how to use `{{ git_auth_secret }}` to enable authenticated Git operations with the `git` resolver, helping you avoid rate-limiting and access private resources securely.

### How It Works

For each `PipelineRun`, Pipelines-as-Code performs these actions automatically:

1. **Generates a Token**: It creates a short-lived, scoped token for your Git provider.
2. **Creates a Secret**: It creates a `Secret` in the target namespace to hold the token. The secret name is unique for each run (e.g., `pac-gitauth-owner-repo-xxxx`, where `xxxx` is a unique suffix generated for the run, typically consisting of random characters or a hash).
3. **Injects the Variable**: It makes the secret's name available in your `.tekton/` templates via the `{{ git_auth_secret }}` variable.

This secret is owned by the `PipelineRun` and is automatically garbage-collected when the `PipelineRun` is deleted. You can learn more about this mechanism in the [Private Repositories documentation]({{< relref "/docs/guide/privaterepo.md" >}}).

### Step 1: Design Your Pipeline for Authentication

First, ensure your `Pipeline` is designed to accept a secret name as a parameter. The `taskRef` using the `git` resolver must be configured to use this parameter for authentication.

Your `pipeline.yaml` should look like this:

```yaml
---
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: my-pipeline
spec:
  params:
    # This parameter will receive the secret name from the PipelineRun via the PaC `{{ git_auth_secret }}` variable
    - name: git-auth-secret
      description: The name of the Kubernetes secret for Git authentication.
      type: string

    # Other parameters for your pipeline
    - name: git-repo-url
      type: string
    - name: git-revision
      type: string
  tasks:
    - name: fetch-remote-task
      taskRef:
        resolver: git
        params:
          - name: url
            value: $(params.git-repo-url)
          - name: revision
            value: $(params.git-revision)
          - name: pathInRepo
            value: path/to/your/task.yaml
          # --- Authentication Parameters ---
          # Use the pipeline parameter to reference the secret name
          - name: http-auth-secret
            value: $(params.git-auth-secret)
```

### Step 2: Use `{{ git_auth_secret }}` in Your PipelineRun

You do not need to create any secrets manually. Simply reference the PaC variable `{{ git_auth_secret }}` in your `PipelineRun` template file (e.g., `.tekton/pipelinerun.yaml`).

PaC will substitute this placeholder with the name of the auto-generated secret at runtime.

```yaml
# .tekton/pipelinerun.yaml

apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: my-pipelinerun-
spec:
  pipelineRef:
    name: my-pipeline
  params:
    # Pass the PaC variable to your pipeline's parameter
    - name: git-auth-secret
      value: "{{ git_auth_secret }}"

    # Pass other necessary parameters
    - name: git-repo-url
      value: "{{ repo_url }}"
    - name: git-revision
      value: "{{ revision }}"
```

By following this pattern, your remote tasks will be fetched using an authenticated session managed entirely by Pipelines-as-Code.

### Beyond Task Resolution

The `{{ git_auth_secret }}` is versatile. Besides its use with the `git` resolver, it can also be used for:

- **Cloning private repositories**: Use the secret as a `workspace` for the `git-clone` task.
- **Calling the Git provider API**: Use the token within the secret to make API calls, for example, to post a comment back to a pull request.

For more examples and details, see the documentation on [Authoring PipelineRuns]({{< relref "/docs/guide/authoringprs.md#using-the-temporary-github-app-token-for-github-api-operations" >}}).
