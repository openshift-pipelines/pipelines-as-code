# Pipelines as Code

[![Container Repository on Quay](https://quay.io/repository/openshift-pipeline/pipelines-as-code/status "Container Repository on Quay")](https://quay.io/repository/openshift-pipeline/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code) [![Go Report Card](https://goreportcard.com/badge/google/ko)](https://goreportcard.com/report/openshift-pipelines/pipelines-as-code)

Pipelines as Code -- An opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as Code let you use the pipelines as Code flow directly with OpenShift
Pipelines.

The Pipelines as Code technique can be described in this web page
[https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code) it allows you
to have your pipelines "sits and live" inside the same repository where your
code is.

The goal of Pipelines as Code is to let you define your
[Tekton](https://tekton.cd) templates inside your source code repository and
have the pipeline run and report the status of the execution when triggered by a
Pull Request or a branch push.

See a walkthought video about it here :

[![Pipelines as Code Walkthought](https://img.youtube.com/vi/Uh1YhOGPOes/0.jpg)](https://www.youtube.com/watch?v=Uh1YhOGPOes)

## Components

Pipelines as Code is built on the following technologies:

- [Tekton Triggers](github.com/tektoncd/triggers): A Tekton Triggers
  EventListener is spun up in a central namespace (`pipelines-as-code`). The
  EventListener is the service responsible for listening to webhook events and acting upon it.

- Repository CRD: The Repository CRD is a new API introduced in the Pipelines as
  Code project. This CRD is used to define the association between the source
  code repository and the Kubernetes namespace in which the corresponding
  Pipelines are run.

- Web VCS support. When iterating over a Pull Request, status and control is
  done on the platform.

  *GitHub*:

  - Support for Checks API to set the status of a PipelineRun.
  - Support rechecks on UI.
  - Support for Pull Request events.
  - Use GitHUB blobs and objects API to get configuration files directly.
    (instead of checking the repo locally)

## Installation

Please follow [this document](INSTALL.MD) here for the install process

## User usage

### GitHub apps Configuration

- Admin gives the GitHub application url to add to the user.
- User clicks on it and add the app on her repository which is in this example
  named `linda/project`
- Users create a namespace inside their kubernetes where the runs are going to
  be executed. i.e:

```bash
kubectl create ns my-pipeline-ci
```

### Namespace Configuration

User create a CustomResource definition inside the namespace `my-pipeline-ci`

```yaml
cat <<EOF|kubectl create -n my-pipeline-ci -f-
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: scratch-my-back
spec:
  url: "https://github.com/linda/project"
  branch: "main"
  namespace: "my-pipeline-ci"
EOF
```

This will match all Pull Request coming to `github.com/linda/project` on branch
main into the namespace `my-pipeline-ci`

For security reasons, the Repository CR needs to be created
in the namespace where Tekton Pipelines associated with the source code repository would be executed.

There is another optional layer of security where PipelineRun can have an
annotation to explicitely target a specific namespace. It would stilll need to
have a Repository CRD created in that namespace to be able to be matched.

With this annotation there is no way a bad actor on a cluster can hijack the
pipelineRun execution to a namespace they don't have access to. To use that
feature you need to add this annotation :

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

and Pipelines as Code will only match the repository in the mynamespace Namespace
instead of trying to match it from all available repository on cluster.

### Writting Tekton pipelines in `.tekton/` directory

- Pipelines as Code tries to be as close to the tekton template as possible.
  Usually you would write your template and save them with a ".yaml" extension  and
  Pipelines as Code will run them.

- Inside your pipeline you would need to be able to consume the commit as received from the
  webhook by checking it out the repository from that ref. You would usually use
  the [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from catalog for the same. To be able to specify those parameters, Pipelines
  as Code allows you to have those two variables filled between double brackets,
  i.e: `{{ var }}`:

  - `{{repo_url}}`: The repository URL of this commit
  - `{{revision}}`: The revision of the commit.

- You need at least one `PipelineRun` with a `PipelineSpec` or a separated
  `Pipeline` object. You can have embedded `TaskSpec` inside
  `Pipeline` or you can have them defined separately as `Task`.

#### Examples

`Pipelines as code` test itself, you can see the examples in its [.tekton](.tekton/) repository.

#### Event matching to a Pipeline

Each `PipelineRun` can match different vcs events via some special annotations
on the `PipelineRun`. For example when you have these metadatas in your `PipelineRun`:

```yaml
 metadata:
    name: pipeline-pr-main
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines as Code` will match the piplinerun `pipeline-pr-main` if the VCS
events target the branch `main` and it's coming from a `[pull_request]`

Multiple target branch can be specified separated by comma, i.e:

`[main, release-nightly]`

You can match on `pull_request` events as above and you can as well match
pipelineRuns on `push` events to a repository

For example this will match the pipeline when there is a push to a commit in
the `main` branch :

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

This will match the pipeline `pipeline-push-on-1.0-tags` when you push the 1.0 tags
into your repository.

Matching annotations are currently mandated or `Pipelines as Code` will not
match your `PiplineRun`.

If there is multiple pipeline matching an event, it will match the first one.
We are currently not supporting multiple PipelineRuns on a single event but
this may be something we can consider to implement in the future.

#### PipelineRuns Cleanups

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

#### Pipelines as Code resolver

If `Pipelines as Code` sees a PipelineRun with a reference to a `Task` or a
`Pipeline`, it will tries to *resolves* it as a single PipelineRun with an embedded `PipelineSpec` to a `PipelineRun`.

It will as well transform the Pipeline Name  to a `generateName`
based on the Pipeline name as well.

This allows you to have multiple runs in the same namespace from the same
PipelineRun with no risk of conflicts.

Everything that runs your pipelinerun and its references need to be inside the
`.tekton/` directory or referenced via a remote task (see below on how the remote
tasks are referenced).

If pipelines as code cannot resolve the referenced tasks in the `Pipeline` or `PipelineSpec` it will fails before applying the pipelinerun onto the cluster.

If you need to test your `PipelineRun` locally before sending it in a PR, you can use
the `resolve` command from the `tkn-pac` CLI See the `--help` of the command to learn about
how to use it.

#### Remote Task support

`Pipelines as Code` support fetching remote tasks from remote location via
annotations on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task via its name in a
Pipeline or a PipelineSpec it will automatically inlines it.

An annotation to a remote task looks like this :

  ```yaml
  pipelinesascode.tekton.dev/task: "[git-clone]"
  ```

this installs the [git-clone](https://github.com/tektoncd/catalog/tree/main/task/git-clone) task from the [tekton hub](https://hub.tekton.dev) repository via its API.

You can have multiple tasks in there if you separate  them by a comma `,`:

```yaml
pipelinesascode.tekton.dev/task: "[git-clone, golang-test, tkn]"
```

You can have multiple lines if you add a `-NUMBER` suffix to the annotation, for example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone]"
  pipelinesascode.tekton.dev/task-1: "[golang-test]"
  pipelinesascode.tekton.dev/task-2: "[tkn]"
```

By default `Pipelines as Code` will interpret the string as the `latest` task to grab from [tekton hub](https://hub.tekton.dev).

If you want to have a specific version of the task, you can add a colon `:` to the string and a version number, like in this example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # will install git-clone 0.1 from tekton.hub
  ```

If you have a string starting with http:// or https://, `Pipelines as Code`
will fetch the task directly from that remote url :

```yaml
  pipelinesascode.tekton.dev/task: "[https://raw.githubusercontent.com/tektoncd/catalog/main/task/git-clone/0.3/git-clone.yaml]"
```

Additionally you can as well a reference to a task from a yaml file inside your repo if you specify the relative path to it, for example :

  ```yaml
  pipelinesascode.tekton.dev/task: "[.tekton/tasks/git-clone.yaml]"
  ```

will grab the `.tekton/tasks/git-clone.yaml` from the current repository on the `SHA` where the event come from (i.e: the current pull request or the current branch push).

If there is any error fetching those resources, `Pipelines as Code` will error out and not process the pipeline.

If the object fetched cannot be parsed as a Tekton `Task` it will error out.

### Running the Pipeline

- A user create a Pull Request.

- If the user sending the Pull Request is not the owner of the repository or not a public member of the organization where the repository belong to, `Pipelines as Code` will not run.

- If the user sending the Pull Request is inside an OWNER file located in the repository root in the main branch (the main branch as defined in the Github configuration for the repo) in the `approvers` or `reviewers` section like this :

```yaml
approvers:
  - approved
```

then the user `approved` will be allowed.

If the sender of a PR is not allowed to run CI but one of allowed user issue a `/ok-to-test` in any line of a comment the PR will be allowed to run CI.

If the user is allowed, `Pipelines as Code` will start creating the `PipelineRun` in the target user namespace.

The user can follow the execution of your pipeline with the
[tkn](https://github.com/tektoncd/cli) cli :

```bash
tkn pr logs -n my-pipeline-ci -Lf
```

Or via your kubernetes UI like the OpenShift console inside your namespace to follow the pipelinerun execution.

### Status

#### GitHub

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

Status of  your pipeline execution is stored inside the Repo CustomResource :

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

Notifications are not enabled natively by Pipelines as Code, the only place
where we report status is when we do a Pull Request and update for example the
Github checks interface with the results.

You can easily configure your pipeline to send a slack message on failures (or
success) with this task :

<https://github.com/chmouel/tekton-slack-task-status>

Pipelines as Code reuse this for its push pipeline, see it here :

[.tekton/push.yaml](.tekton/push.yaml)

## CLI

`Pipelines as Code` provide a CLI which is design to work as tkn plugin. See the
[INSTALL.md](INSTALL.md) to see how to install it.

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

for example to list all repo status in your system (if you have the right for it) :

```shell
tkn pac repo ls --all-namespaces
```

and to dig into a specific repository status:

```shell
tkn pac repo desc repository-name -n namespace
```

`tkn-pac` is as well available inside the container image :

or from the container image user docker/podman:

```shell
docker run -e KUBECONFIG=/tmp/kube/config -v ${HOME}/.kube:/tmp/kube \
     -it quay.io/openshift-pipeline/pipelines-as-code tkn-pac help
```

and here is a short walk-thought video :

<https://user-images.githubusercontent.com/98980/123408765-ac32c900-d5ad-11eb-83bf-6ef068a787bb.mp4>
