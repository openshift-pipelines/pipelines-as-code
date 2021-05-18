[![Container Repository on Quay](https://quay.io/repository/openshift-pipeline/pipelines-as-code/status "Container Repository on Quay")](https://quay.io/repository/openshift-pipeline/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code)

# Pipelines as Code

Pipelines as Code, an opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as Code let you use the pipelines as Code flow directly with OpenShift
Pipelines.

Pipelines as Code technique can be described in this web page
[https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code) it allows you
to have your pipelines "sits and live" inside the same repository where your
code is.

The goal of Pipelines as Code is to let you write your
[Tekton](https://tekton.cd) templates within your repository and let Pipelines
as Code runs and reports the pipeline status on Pull Request or branch push.

## Components

Pipelines as Code leverage on this technologies :

- [Tekton Triggers](github.com/tektoncd/triggers): A Tekton Triggers
  EventListenner is spun up in a central namespace (`pipelines-as-code`). The
  EventListenner listen for webhook events and acts upon it.

- Repository CRD: A new CRD introduced with Pipelines as Code, It allows the
  user to specify which repo is started in which namespace they want.

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

User create a CustomResource definition inside the namespace my-pipeline-ci

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

For security reasons you need to make sure that the Repository CR is installed
into the same namespace are where we want to execute them.

### Write Tekton pipeline in `.tekton/` directory

- Pipelines as Code tries to be as close to the tekton template as possible.
  Usually you write your template and save them with a ".yaml" extension  and
  Pipelines as Code will run them.

- Inside your pipeline you need to be able the commit as received from the
  webhook by checking it out the repository from that ref. You usually will use
  the [git-clone](https://github.com/tektoncd/catalog/blob/main/task/git-clone/)
  task from catalog for this. To be able to specify those parameters, Pipelines
  as Code allows you to have those two variables filled between double brackets,
  i.e: `{{ var }}`:

  - `{{repo_url}}`: The repository URL of this commit
  - `{{revision}}`: The revision of the commit.

- You need at least one `PipelineRun` with a `PipelineSpec` or a separated
  `Pipeline` object. You can have embedded `TaskSpec` inside
  `Pipeline` or you can have them defined separately as `Task`.

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

If there is multiple pipeline matching the event, it will match the first one.

You can match on `pull_request` events as above and on git `push` events to a
repo.

For example this will match the pipeline when there is a push to a commit in
the `main` branch :

```yaml
 metadata:
  name: pipeline-push-on-main
  annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/heads/main]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```


You need to specify the refs and not just the branch, this allows to as well
match on tags. For example :

```yaml
 metadata:
 name: pipeline-push-on-1.0-tags
 annotations:
    pipelinesascode.tekton.dev/on-target-branch: "[refs/tags/1.0]"
    pipelinesascode.tekton.dev/on-event: "[push]"
```

will match the pipeline `pipeline-push-on-1.0-tags` when you push the 1.0 tags
into your repository.

Matching annotations are currently mandated or `Pipelines as Code` will not
match your `PiplineRun`.

#### Pipelines asCode resolver

If `Pipelines as Code` sees multiple documents, it tries to *resolves* it as a
  single PiplineSpec embedded to a `PipelineRun`. It will add a `generateName`
  based on the Pipeline name as well. This allows you to have multiple runs in
  the same namespace without risk of conflicts.

Everything that runs your pipelinerun should be self contained inside the
`.tekton/` directory or from some remote tasks (see below).  If pipelines as
code cannot resolve the referenced tasks in the `Pipeline` or `PipelineSpec`
it will fails before applying the pipelinerun onto the cluster.

#### Remote Task support

`Pipelines asCode` support fetching remote tasks from remote location via  annotation on PipelineRun.

If the resolver sees a PipelineRun referencing a remote task via its name in a  Pipeline or a PipelineSpec it will automatically inlines it.

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

If you have a string starting with http:// or https://, `Pipelines asCode`
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

A user create a Pull Request.

If the user sending the Pull Request is not the owner of the repository or not a public member of the organization where the repository belong to, `Pipelines as Code` will not run.

If the user sending the Pull Request is inside an OWNER file located in the repository root in the main branch (the main branch as defined in the Github configuration for the repo) in the `approvers` or `reviewers` section like this :

```yaml
approvers:
  - approved
```

then the user `approved` will be allowed.

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
the Pipeline.

#### CRD

Status of  your pipeline execution is stored inside the Repo CustomResource :

```bash
% kubectl get repo -n pipelines-ascode-ci
NAME                  URL                                                        NAMESPACE             SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipelines-ascode-ci   https://github.com/openshift-pipelines/pipelines-as-code   pipelines-ascode-ci   True        Succeeded   59m         56m
```

The last 5 status are stored inside the CustomResource and can be accessed
directly like this :

```json
% kubectl get repo -n pipelines-ascode-ci -o json|jq .items[].pipelinerun_status
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
kubectl apply -f https://uploader-cron.svc.ci.openshift.org/pipelines-as-code/release-nightly.yaml
```

which will apply the release.yaml to your kubernetes cluster, creating the
namespace, the roles and all other bits needed.

You will need to have events coming through to your EventListenner so follow
the next steps on how to do that.

### Github configuration

To setup Pipelines as Code on GitHUB you need a GitHUB Apps created.

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
% kubectl -n openshift-pipelines-ascode create secret generic github-app-secret \
        --from-literal private.key="$(cat /tmp/github.app.key)"
        --from-literal application_id="APPLICATION_ID_NUMBER" \
        --from-literal webhook.secret="WEBHOOK_SECRET"
```

This secret is used to generate a token on behalf of the user running the event
and make sure to validate the webhook via the webhook secret.
