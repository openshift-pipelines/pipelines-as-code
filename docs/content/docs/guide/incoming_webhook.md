---
title: Incoming Webhook
---
# Incoming webhook

Pipelines-as-Code support the concept of incoming webhook URL. It let you
trigger PipelineRun in a Repository using a shared secret and URL,
instead of creating a new code iteration.

## Incoming Webhook URL

To use incoming webhooks in Pipelines-as-Code, you must configure the
incoming field in your Repository CRD. This field references a `Secret`, which
serves as the shared secret, as well as the branches targeted by the incoming
webhook. Once configured, Pipelines-as-Code will match `PipelineRuns` located in
your `.tekton` directory if the `on-event` annotation of the targetted pipelinerun is
targettting a push or incoming event.

{{< hint info >}}
If you are not using the github app provider (ie: webhook based provider) you
will need to have a `git_provider` spec to specify a token.

Additionally since we are not able to detect automatically the type of provider
on URL. You will need to add it to the `git_provider.type` spec. Supported
values are:

- github
- gitlab
- bitbucket-cloud

Whereas for `github-apps` this doesn't need to be added.
{{< /hint >}}

### GithubApp

The example below illustrates the use of GithubApp to trigger a PipelineRun
based on an incoming webhook URL.

The Repository Custom Resource (CR) specifies the target branch as
main and includes an incoming webhook URL with a shared password stored in a
Secret called `repo-incoming-secret`:

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  incoming:
    - targets:
      - main
      secret:
        name: repo-incoming-secret
      type: webhook-url
```

A PipelineRun is then annotated to target the incoming event and the main branch:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: target_pipelinerun
  annotations:
    pipelinesascode.tekton.dev/on-event: "[incoming]"
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
```

A secret called repo-incoming-secret is utilized as a shared password to ensure
that only authorized users can initiate the `PipelineRun`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-incoming-secret
  namespace: ns
type: Opaque
stringData:
  secret: very-secure-shared-secret
```

After setting this up, you will be able to start the PipelineRun with a POST
request sent to the controller URL appended with /incoming. The request
includes the very-secure-shared-secret, the repository name (repo), the target
branch (main), and the PipelineRun name (target_pipelinerun).

As an example here is a curl snippet starting the PipelineRun:

```shell
curl -X POST 'https://control.pac.url/incoming?secret=very-secure-shared-secret&repository=repo&branch=main&pipelinerun=target_pipelinerun'
```

in this snippet, note two things the `"/incoming"` path to the controller URL
and the `"POST"` method to the URL rather than a simple `"GET"`.

It is important to note that when the PipelineRun is triggered, Pipelines as
Code will treat it as a push event and will have the capabilty to report the
status of the PipelineRuns. To obtain a report or a notification, a finally
task can be added directly to the Pipeline, or the Repo CRD can be inspected
using the tkn pac CLI. The [statuses](/docs/guide/statuses) documentation
provides guidance on how to achieve this.

### Webhook methods (GitHub Webhook, Gitlab, Bitbucket etc..)

Here is an example of a Repository CRD matching the target branch main:

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  git_provider:
    type: github
    secret:
      name: "owner-token"
  incoming:
    - targets:
      - main
      secret:
        name: repo-incoming-secret
      type: webhook-url
```

As noted in the section above, you need to specify a incoming secret  inside
the `repo-incoming-secret` Secret.
