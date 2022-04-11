---
title: Private Repositories
weight: 7
---
# Private repositories

Pipelines as Code support private repositories by creating or updating a secret
in the target namespace with the user token for the
[git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone) task
to use and be able to clone private repositories.

Whenever Pipelines as Code create a new PipelineRun in the target namespace it
will create or update a secret called :

`pac-git-basic-auth-REPOSITORY_OWNER-REPOSITORY_NAME`

The secret contains a `.gitconfig` and a Git credentials `.git-credentials` with
the https URL using the token it discovered from the GitHub application or
attached to the secret.

As documented :

<https://github.com/tektoncd/catalog/blob/main/task/git-clone/0.4/README.md>

the secret needs to be referenced inside your PipelineRun and Pipeline as a
workspace called basic-auth to be passed to the `git-clone` task.

For example in your PipelineRun you will add the workspace referencing the
Secret :

```yaml
  workspace:
  - name: basic-auth
    secret:
      secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"
```

And inside your pipeline, you are referencing them for the `git-clone` to reuse  :

```yaml
[…]
workspaces:
  - name basic-auth
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

The `git-clone` task will pick up the basic-auth (optional) workspace and
automatically use it to be able to clone the private repository.

You can see as well a full example [here](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/test/testdata/pipelinerun_git_clone_private.yaml)

This behavior can be disabled by configuration, setting the `secret-auto-create` to false or true
inside the [Pipelines-as-Code Configmap](/docs/install/settings#pipelines-as-code-configuration-settings).
