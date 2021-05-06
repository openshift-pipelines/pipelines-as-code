[![Container Repository on Quay](https://quay.io/repository/openshift-pipeline/pipelines-as-code/status "Container Repository on Quay")](https://quay.io/repository/openshift-pipeline/pipelines-as-code) [![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code)

# Pipelines as Code

Pipelines as code, an opinionated CI based on OpenShift Pipelines / Tekton.

## Introduction

Pipelines as code let you use the pipelines as code flow directly with OpenShift
Pipelines.

Pipelines as code technique can be described in this web page
[https://www.thoughtworks.com/radar/techniques/pipelines-as-code](https://www.thoughtworks.com/radar/techniques/pipelines-as-code) it allows you
to have your pipelines "sits and live" inside the same repository where your
code is.

The goal of Pipelines as Code is to let you write your
[Tekton](https://tekton.cd) templates within your repository and let Pipelines
as Code runs and reports the pipeline status on Pull Request or branch push.

## Components

Pipelines as code leverage on this technologies :

- [Tekton Triggers](github.com/tektoncd/triggers): A Tekton Triggers
  EventListenner is spun up in a central namespace (`pipelines-as-code`). The
  EventListenner listen for webhook events and acts upon it.

- Repository CRD: A new CRD introduced with Pipelines as code, It allows the
  user to specify which repo is started in which namespace they want.

- Web VCS support. When iterating over a Pull Request, status and control is
  done on the platform.

  GitHub:

  - Support for Checks API to set the status of a PipelineRun.
  - Support rechecks on UI.
  - Support for Pull Request events.
  - Use GitHUB blobs and objects API to get configuration files directly.
    (instead of checking the repo locally)

## Usage

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

- Pipelines as code tries to be as close to the tekton template as possible.
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
  `Pipeline` object. You can have as many yaml files as you want but only one
  pipeline by repo is supported. You can have embedded `TaskSpec` inside
  `Pipeline` or you can have them defined separately as `Task`.

- If `Pipelines as Code` sees multiple documents, it tries to *resolves* it as a
  single PiplineSpec embedded to a `PipelineRun`. It will add a `generateName`
  based on the Pipeline name as well. This allows you to have multiple runs in
  the same namespace without risk of conflicts.

- Everything that runs your pipeline should be self contained inside the
  `.tekton/` directory including the tasks. Optionally it supports a file called
  `tekton.yaml` which allows you to integrate *remote* tasks directly on your
  pipeline. For example if you have this :

  ```yaml
  tasks:
    - https://task.com/task1.yaml
    - task/dir/task2.yaml
    - git-clone
    - buildah:0.2
  ```

  - Everything that starts with https:// will be retrieved remotely.
  - Everything that has a slash (/) into it will be retrieved from inside the repository.
  - Everything else is a remote task from [the hub](https://hub.tekton.dev/). It
    will grab the latest from Tekton hub unless there is `:VERSION_NUMBER` which
    means we want that specific `version_number` to be grabbed for that task.

  All those tasks will be resolved inside your Pipeline, so if you ref to it
  from your pipeline they will be embedded directly into your `PipelineRun`
  after being retrieved.

### Running the Pipeline

You simply send your Pull Request and Pipelines as Run will apply your pipeline
and run it.

You can follow the execution of your pipeline with the
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
