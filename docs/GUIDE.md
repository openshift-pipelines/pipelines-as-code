# Pipeline as Code - Usage Guide

## Repository CRD

The purposes of the Repository CRD  is :

- To let know to Pipelines as Code that this event from this URL needs to be handled.
- To tell Pipelines as Code on which namespace the PipelineRuns are going to be executed.
- To reference a api secret, username or api URL if needed for the git provider
  platforms that requires it (ie: when you are using webhooks method and not
  the github application).
- To give the last Pipelinerun status for that Repository (5 by default).

The flow looks like this :

Via the tkn pac CLI or other method the user creates a `CustomResourceDefinition`
inside the target namespace `my-pipeline-ci` :

```yaml
cat <<EOF|kubectl create -n my-pipeline-ci -f-
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: scratch-my-back
spec:
  url: "https://github.com/linda/project"
EOF
```

Whenever there is a event coming from `github.com/linda/project` Pipelines as
Code will match it and starts checking out the content of the `linda/project`
for pipelinerun to match in the `.tekton/` directory.

The Repository CRD needs to be created in the namespace where Tekton Pipelines
associated with the source code repository would be executed, it cannot target
another namespace.

If there is multiples CRD matching the same event, only the oldest one will
match. If you need to match a specific namespace you would need to use the
target-namepsace feature in the pipeine annotation (see below).

There is another optional layer of security where PipelineRun can have an
annotation to explicitely target a specific
namespace. It would stilll need to have a Repository CRD created in that
namespace to be able to be matched.

With this annotation a bad actor on a cluster cannot hijack the pipelineRun
execution to a namespace they don't have access to. To use that feature you
need to add this annotation to the pipeline annotation :

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

and Pipelines as Code will only match the repository in the mynamespace
Namespace instead of trying to match it from all available repository on cluster.

## Authoring PipelineRun in `.tekton/` directory

- Pipelines as Code will always try to be as close to the tekton template as
  possible. Usually you will write your template and save them with a ".yaml"
  extension and Pipelines as Code will run them.

- Inside your pipeline you need to be able to check out the commit as
  received from the webhook by checking it out the repository from that ref. You
  would usually use the
  [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from catalog.

  To be able to specify the parameters of your commit and url, Pipelines as Code
  allows you to have those "dynamic" variables expanded. Those variables look
  like this `{{ var }}`and those are the one you can use:

  - `{{repo_owner}}`: The repository owner.
  - `{{repo_name}}`: The repository name.
  - `{{repo_url}}`: The repository full URL.
  - `{{revision}}`: The commit full sha revision.

- You need at least one `PipelineRun` with a `PipelineSpec` or a separated
  `Pipeline` object. You can have embedded `TaskSpec` inside
  `Pipeline` or you can have them defined separately as `Task`.

### Examples

`Pipelines as code` test itself, you can see the examples in its
[.tekton](.tekton/) repository.

### Event matching to a Pipeline

Each `PipelineRun` can match different git provider events via some special
annotations on the `PipelineRun`. For example when you have these metadatas in
your `PipelineRun`:

```yaml
 metadata:
    name: pipeline-pr-main
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines as Code` will match the piplinerun `pipeline-pr-main` if the git
provider events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branch can be specified separated by comma, i.e:

`[main, release-nightly]`

You can match on `pull_request` events as above and you can as well match
pipelineRuns on `push` events to a repository

For example this will match the pipeline when there is a push to a commit in the
`main` branch :

```yaml
 metadata:
  name: pipeline-push-on-main
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/heads/main]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

You can specify the full refs like `refs/heads/main` or the shortref like
`main`. You can as well specify globs, for example `refs/heads/*` will match any
target branch or `refs/tags/1.*` will match all the tags starting from `1.`.

A full example for a push of a tag :

```yaml
 metadata:
 name: pipeline-push-on-1.0-tags
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/tags/1.0]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

This will match the pipeline `pipeline-push-on-1.0-tags` when you push the 1.0
tags into your repository.

Matching annotations are currently mandated or `Pipelines as Code` will not
match your `PipelineRun`.

If there is multiple pipeline matching an event, it will match the first one. We
are currently not supporting multiple PipelineRuns on a single event but this
may be something we can consider to implement in the future.

## PipelineRuns Cleanups

There can be a lot of PipelineRuns into an user namespace and Pipelines as Code
has the ability to only keep a number of PipelineRuns that matches an event.

For example if the PipelineRun has this annotation :

```yaml
pipelinesascode.tekton.dev/max-keep-runs: "maxNumber"
```

Pipelines as Code sees this and will start cleaning up right after it finishes a
successful execution keeping only the maxNumber of PipelineRuns.

It will skip the `Running` PipelineRuns but will not skip the PipelineRuns with
`Unknown` status.

## Private repositories

Pipelines as Code support private repositories by creating or updating a secret
in the target namespace with the user token for the
[git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone) task
to use and be able to clone private repositories.

Whenever Pipelines as Code create a new PipelineRun in the target namespace it
will create or update a secret called :

`pac-git-basic-auth-REPOSITORY_OWNER-REPOSITORY_NAME`

The secret contains a `.gitconfig` and a git credentials `.git-credentials` with
the https url using the token it discovered from the Github application or
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

And inside your pipeline, you are referencing them for the git-clone to reuse  :

```yaml
[...]
workspaces:
  - name basic-auth
params:
    - name: repo_url
    - name: revision
[...]
tasks:
  workspaces:
    - name: basic-auth
      workspace: basic-auth
  [...]
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

The git-clone task will pick up the basic-auth (optional) workspace and
automatically use it to be able to clone the private repository.

You can see as well a full example [here](./test/testdata/pipelinerun_git_clone_private.yaml)

This behaviour can be disabled by configuration the `secret-auto-create` key
inside the [Pipelines-as-Code Configmap](./INSTALL.md#configuration).

## Pipelines as Code resolver

If `Pipelines as Code` sees a PipelineRun with a reference to a `Task` or a
`Pipeline`, it will tries to *resolves* it as a single PipelineRun with an
embedded `PipelineSpec` to a `PipelineRun`.

It will as well transform the Pipeline Name to a `generateName` based on the
Pipeline name as well.

This allows you to have multiple runs in the same namespace from the same
PipelineRun with no risk of conflicts.

Everything that runs your pipelinerun and its references need to be inside the
`.tekton/` directory or referenced via a remote task (see below on how the
remote tasks are referenced).

If pipelines as code cannot resolve the referenced tasks in the `Pipeline` or
`PipelineSpec` it will fails before applying the pipelinerun onto the cluster.

If you need to test your `PipelineRun` locally before sending it in a PR, you
can use the `resolve` command from the `tkn-pac` CLI See the `--help` of the
command to learn about how to use it.

### Remote Task support

`Pipelines as Code` support fetching remote tasks from remote location via
annotations on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task via its name in a
Pipeline or a PipelineSpec it will automatically inlines it.

An annotation to a remote task looks like this :

  ```yaml
  pipelinesascode.tekton.dev/task: "[git-clone]"
  ```

this installs the
[git-clone](https://github.com/tektoncd/catalog/tree/main/task/git-clone) task
from the [tekton hub](https://hub.tekton.dev) repository via its API.

You can have multiple tasks in there if you separate them by a comma `,`:

```yaml
pipelinesascode.tekton.dev/task: "[git-clone, golang-test, tkn]"
```

You can have multiple lines if you add a `-NUMBER` suffix to the annotation, for
example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone]"
  pipelinesascode.tekton.dev/task-1: "[golang-test]"
  pipelinesascode.tekton.dev/task-2: "[tkn]"
```

By default `Pipelines as Code` will interpret the string as the `latest` task to
grab
from [tekton hub](https://hub.tekton.dev).

If you want to have a specific version of the task, you can add a colon `:` to
the string and a version number, like in
this example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # will install git-clone 0.1 from tekton.hub
  ```

If you have a string starting with http:// or https://, `Pipelines as Code`
will fetch the task directly from that remote url :

```yaml
  pipelinesascode.tekton.dev/task: "[https://raw.githubusercontent.com/tektoncd/catalog/main/task/git-clone/0.3/git-clone.yaml]"
```

Additionally you can as well a reference to a task from a yaml file inside your
repo if you specify the relative path to it, for example :

  ```yaml
  pipelinesascode.tekton.dev/task: "[.tekton/tasks/git-clone.yaml]"
  ```

will grab the `.tekton/tasks/git-clone.yaml` from the current repository on the
`SHA` where the event come from (i.e: the current pull request or the current
branch push).

If there is any error fetching those resources, `Pipelines as Code` will error
out and not process the pipeline.

If the object fetched cannot be parsed as a Tekton `Task` it will error out.

## Running the Pipeline

- A user create a Pull Request.

- Pipelines as Code picks the event and matches to a Repo CRD installed on the
  cluster.

- The user would only be allowed to run the CI if :
  - The user is the owner of the repository.
  - The user is a collaborator on the repository.
  - The user is a public member on the organization of the repository.

- If the user is sending the Pull Request is inside an OWNER file located in the
  repository root on the main branch (the main branch as defined in the Github
  configuration for the repo) and added to either `approvers` or `reviewers`
  sections like this :

```yaml
approvers:
  - approved
```

then the user `approved` will be allowed.

- If the sender of a PR is not allowed to run CI but one of allowed user issue a
  `/ok-to-test` in any line of a comment the PR will be allowed to run CI.

- If the user is allowed, `Pipelines as Code` will start creating the
`PipelineRun` in the target user namespace.

- The user can follow the execution of your pipeline with the
[tkn](https://github.com/tektoncd/cli) cli :

```bash
tkn pr logs -n my-pipeline-ci -Lf
```

Or with the OpenShift console inside your namespace to follow the pipelinerun
execution via the URL provided on the "Checks" tab if you run with Github App.

## Status

### GitHub

When the pipeline finishes the status will be added in the Github Check tabs
with a short recap of how long each task of your pipeline took and the output of
`tkn pr describe`.

If there was a failure you can click on the "Re-Run" button on the left to rerun
the Pipeline or you can issue a issue comment with a line starting and finishing
with the string `/retest` to ask Pipelines as Code to retest the current PR.

Example :

```text
Thanks for contributing! This is a much needed bugfix! ❤️
The failure is not with your PR but seems to be an infra issue.

/retest
```

#### CRD

Status of your pipeline execution is stored inside the Repo CustomResource :

```bash
% kubectl get repo -n pipelines-as-code-ci
NAME                  URL                                                        NAMESPACE             SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipelines-as-code-ci   https://github.com/openshift-pipelines/pipelines-as-code   pipelines-as-code-ci   True        Succeeded   59m         56m
```

The last 5 status are stored inside the CustomResource and can be accessed
directly like this :

```json
% kubectl get repo -n pipelines-as-code-ci -o json|jq .items[].pipelinerun_status
[
  {
    "completionTime": "2021-05-05T11:00:05Z",
    "conditions": [
      {
        "lastTransitionTime": "2021-05-05T11:00:05Z",
        "message": "Tasks Completed: 3 (Failed: 0, Cancelled 0), Skipped: 0",
        "reason": "Succeeded",
        "status": "True",
        "type": "Succeeded"
      }
    ],
    "pipelineRunName": "pipelines-as-code-test-run-7tr84",
    "startTime": "2021-05-05T10:53:43Z"
  },
  {
    "completionTime": "2021-05-05T11:20:18Z",
    "conditions": [
      {
        "lastTransitionTime": "2021-05-05T11:20:18Z",
        "message": "Tasks Completed: 3 (Failed: 0, Cancelled 0), Skipped: 0",
        "reason": "Succeeded",
        "status": "True",
        "type": "Succeeded"
      }
    ],
    "pipelineRunName": "pipelines-as-code-test-run-2fhhg",
    "startTime": "2021-05-05T11:11:20Z"
  },
  [...]
```

### Notifications

Notifications is not handled by Pipelines as Code, the only place where we
notify a status in a interface is when we do a Pull Request on for example the
Github checks interface to show the results of the pull request.

If you need some other type of notification you can use
the [finally feature of tekton pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md#adding-finally-to-the-pipeline)
.

Here is an example task to send a slack message on failures (or success if you
like) :

<https://github.com/chmouel/tekton-slack-task-status>

The push pipeline of Pipelines as Code use this task, you can see the example
here :

[.tekton/push.yaml](https://github.com/openshift-pipelines/pipelines-as-code/blob/7b41cc3f769af40a84b7ead41c6f037637e95070/.tekton/push.yaml#L116)

## CLI

`Pipelines as Code` provide a CLI which is design to work as tkn plugin. See the
installation instruction in the [INSTALL.md](./INSTALL.md#CLI)
documentation

### CLI commands

```shell
tkn pac help
```

You can easily create a new repo CR with the command :

```shell
tkn pac repo create
```

It will detect your current GIT repo URL and branch your current namespace and
ask you for a target, it will ask you if you want to create sample pipelinerun
for you to customize it.

for example to list all repo status in your system (if you have the right for
it) :

```shell
tkn pac repo ls --all-namespaces
```

and to dig into a specific repository status:

```shell
tkn pac repo desc repository-name -n namespace
```
