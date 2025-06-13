---
title: Private Repositories
weight: 7
---
# Private repositories

Pipelines-as-Code allows the use of private repositories by creating or
updating a secret in the target namespace. This secret contains the user token
required for the [git-clone](https://hub.tekton.dev/tekton/task/git-clone) task
to clone private repositories.

Whenever Pipelines-as-Code creates a new PipelineRun in the target namespace,
it also creates a secret with a specific name format:

`pac-gitauth-REPOSITORY_OWNER-REPOSITORY_NAME-RANDOM_STRING`

This secret contains a [Git Config](https://git-scm.com/docs/git-config) file named
`.gitconfig` and a [Git credentials](https://git-scm.com/docs/gitcredentials)
file named `.git-credentials`. These files configure the base HTTPS URL of the git provider
(such as <https://github.com>) using the token obtained from the GitHub application
or from a secret attached to the repository CR on git provider when using the webhook method.

The secret includes a key referencing the token as a key to let you easily use it in your task for
other provider operations.

See the documentation with example on how to use it
[here](../authoringprs/#using-the-temporary-github-app-token-for-github-api-operations)

The secret has a
[ownerRef](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
field to the created PipelineRun. This means the secret will be auto deleted
when you delete the `PipelineRun` it references to.

{{< hint warning >}}
To disable this behavior, you can configure the `secret-auto-create` setting in
the Pipelines-as-Code Configmap. You can set it to either false or true
depending on your requirements.
{{< /hint >}}

## Using the generated token in your PipelineRun

The git-clone task documentation, which is available at
<https://github.com/tektoncd/catalog/blob/main/task/git-clone/0.4/README.md>,
states that the secret needs to be referred to as a workspace named
"basic-auth" inside your PipelineRun so that it can be passed to
the `git-clone` task.

To achieve this, you can add the workspace referencing the secret in your
PipelineRun. For instance, you can include the following code in your
PipelineRun to reference the Secret:

```yaml
  workspace:
  - name: basic-auth
    secret:
      secretName: "{{ git_auth_secret }}"
```

Once you have added the workspace referencing the secret in your PipelineRun as
described earlier, you can then pass the git-clone task to reuse it inside your
Pipeline or embedded PipelineRun. This is typically achieved by including the
git-clone task as a step in your Pipeline or embedded PipelineRun, and
specifying the workspace name as "basic-auth" in the task definition. Here's an
example of how you could pass the git-clone task to reuse the secret in your
Pipeline:

```yaml
[…]
workspaces:
  - name: basic-auth
params:
    - name: repo_url
    - name: revision
[…]
tasks:
  workspaces:
    - name: basic-auth
      workspace: basic-auth
  […]
  tasks:
  - name: git-clone-from-catalog
      taskRef:
        name: git-clone
      params:
        - name: url
          value: $(params.repo_url)
        - name: revision
          value: $(params.revision)
```

- A full example is available
  [here](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/test/testdata/pipelinerun_git_clone_private.yaml)

## Fetching remote tasks from private repositories

See the [resolver documentation](../resolver/#remote-http-url-from-a-private-github-repository) for more details.
