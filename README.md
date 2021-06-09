[![Container Repository on Quay](https://quay.io/repository/openshift-pipeline/pipelines-as-code/status "Container Repository on Quay")](https://quay.io/repository/openshift-pipeline/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code)

# Pipelines as Code

Pipelines as Code is an opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as Code let you use the pipelines as Code flow directly with OpenShift
Pipelines.

The Pipelines as Code technique can be described in this web page
[https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code) it allows you
to have your pipelines "sits and live" inside the same repository where your
code is.

The goal of Pipelines as Code is to let you define your
[Tekton](https://tekton.cd) templates inside your source code repository and have the pipeline run and report the status of the execution when triggered by a Pull Request or a branch push.

## Components

Pipelines as Code is built on the following technologies :

- [Tekton Triggers](github.com/tektoncd/triggers): A Tekton Triggers
  EventListener is spun up in a central namespace (`pipelines-as-code`). The
  EventListener is the service responsible for listening to webhook events and acting upon it.

- Repository CRD: The Repository CRD is a new API introduced in the Pipelines as
  Code project. This CRD is used to define the association between the source
  code repository and the Kubernetes namespace in which the corresponding
  Pipelines are run.

- Web VCS support. When iterating over a Pull Request, status and control is
  done on the platform.

  GitHub:

  - Support for Checks API to set the status of a PipelineRun.
  - Support rechecks on UI.
  - Support for Pull Request events.
  - Use GitHUB blobs and objects API to get configuration files directly.
    (instead of checking the repo locally)

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

#### Pipelines as Code resolver

If `Pipelines as Code` sees a PipelineRun with a reference to a `Task` or a
`Pipeline`, it will tries to *resolves* it as a single PipelineRun with an embedded `PipelineSpec` to a `PipelineRun`.

It will as well transform the Pipeline Name  to a `generateName`
based on the Pipeline name as well.

This allows you to have multiple runs in the same namespace from the same
PipelineRun with no risk of conflicts.

Everything that runs your pipelinerun and its references neeed to inside the
`.tekton/` directory or referenced as remote tasks (see below on how the remote
tasks are specified).  If pipelines as code cannot resolve the referenced tasks
in the `Pipeline` or `PipelineSpec` it will fails before applying the
pipelinerun onto the cluster.

If you need to test your `PipelineRun` locally before sending it in a PR, you can use
the `tkresolver` CLI, by installing it like this :

```shell
go install github.com/openshift-pipelines/pipelines-as-code/cmd/tknresolve
```

and you can use the tknresolve binary to generate the PipelineRun the say way it
is generated on events. See the `--help` of the `tknresolve` to learn about how
this CLI and on how to use it.

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

You can have multiple lines if you add a `-NUMBER` prefix to the annotation, for example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone]"
  pipelinesascode.tekton.dev/task-1: "[golang-test]"
  pipelinesascode.tekton.dev/task-2: "[tkn]"
```

By default `Pipelines as Code` will interpret the string as the `latest` task to grab from [tekton hub](https://hub.tekton.dev).

If instead you want to have a specific task, you can add a colon `:` to the string and a version number, like in this example :

```yaml
  pipelinesascode.tekton.dev/task: "[git-clone:0.1]" # will install git-clone 0.1 from tekton.hub
  ```

If you have a string starting with http:// or https://, `Pipelines as Code`
will fetch the task directly from that remote url instead of going via the
`tekton hub` :

```yaml
  pipelinesascode.tekton.dev/task: "[https://raw.githubusercontent.com/tektoncd/catalog/main/task/git-clone/0.3/git-clone.yaml]"
```

You can as well a reference to a task from a yaml file inside your repo if you specify the relative path to it, for example :

  ```yaml
  pipelinesascode.tekton.dev/task: "[.tekton/tasks/git-clone.yaml]"
  ```

will grab the `.tekton/tasks/git-clone.yaml` from the current repository on the `SHA` where the event come from (i.e: the current pull request or the current branch push).

If there is any error fetching those resources, `Pipelines as Code` will error out and not process the pipeline.
If the object fetched cannot be parsed as a Tekton `Task` it will error out.

### Running the Pipeline

* A user create a Pull Request.

* If the user sending the Pull Request is not the owner of the repository or not a public member of the organization where the repository belong to, `Pipelines as Code` will not run.

* If the user sending the Pull Request is inside an OWNER file located in the repository root in the main branch (the main branch as defined in the Github configuration for the repo) in the `approvers` or `reviewers` section like this :

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

When the pipeline finishes the status should be added in the Github Check tabs
with a short recap of how long each task of your pipeline took and the output of
`tkn pr describe`.

If there was a failure you can click on the "Re-Run" button on the left to rerun
the Pipeline or you can issue a issue comment with a line starting and finishing
with the string `/retest` to ask Pipeline as Code to retest the current PR.

Example :

```
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

## Setup

You simply need to run this command :

```bash
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml
```

which will apply the release.yaml to your kubernetes cluster, creating the
namespace, the roles and all other bits needed.

You will need to have events coming through to your EventListenner so follow
the next steps on how to do that.

### Github configuration

To setup Pipelines as Code on Github, you need to have a Github App created.

You need the Webhook of the app pointing to your Ingress endpoint which would
then go to the triggers enventlistenner/service.

You need to make sure you have those permissions and events checked on the
GitHub app :

```json
             "default_permissions": {
                 "checks": "write",
                 "contents": "write",
                 "issues": "write",
                 "members": "read",
                 "metadata": "read",
                 "organization_plan": "read",
                 "pull_requests": "write"
             },
             "default_events": [
                 "commit_comment",
                 "issue_comment",
                 "pull_request",
                 "pull_request_review",
                 "pull_request_review_comment",
                 "push"
             ]
```

When you have created the `github-app-secret` Secret, grab the private key the
`application_id` and the `webhook_secret`  from the interface, place the private
key in a file named for example `/tmp/github.app.key` and issue those commands :

```bash
% kubectl -n pipelines-as-code create secret generic github-app-secret \
        --from-literal private.key="$(cat /tmp/github.app.key)"
        --from-literal application_id="APPLICATION_ID_NUMBER" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

This secret is used to generate a token on behalf of the user running the event
and make sure to validate the webhook via the webhook secret.

You will then need to make sure to expose the `EventListenner` via a
[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) or a
[OpenShift
Route](https://docs.openshift.com/container-platform/latest/networking/routes/route-configuration.html)
so GitHub can get send the webhook to it.
