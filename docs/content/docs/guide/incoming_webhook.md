---
title: Incoming Webhook
---
# Incoming webhook

Pipelines as Code support the concept of incoming webhook URL. Which let you
start a PipelineRun in a Repository by a URL and a shared secret rather than
having to generate a new code iteration.

## Incoming Webhook URL

You need to set your incoming match your Repository CRD, in your match you
specify a reference Secret which will be used as a shared secret and the
branches targetted by the incoming webhook.

{{< hint danger >}}
you will need to have a git_provider spec to specify a token when using the
github-apps method the same way we are doing for github-webhook method. Refer to
the [github webhook documentation](/docs/install/github_webhook) for how to set
this up.
{{< /hint >}}

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

a secret named `repo-incoming-secret` will have this value:

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

after setting this up, you will be able to trigger a PipelineRun called
`pipelienrun1` which will be located in the `.tekton` directory of the Git repo
`https://github.com/owner/repo`. As an example here is the full curl snippet:

```shell
curl -X POST https://control.pac.url/incoming?secret=very-secure-secret&repository=repo,branch=main&pipelinerun=target_pipelinerun
```

note two things the `"/incoming"` path to the controller URL and the `"POST"`
method to the URL rather than a simple `"GET"`.

Pipelines as Code when matched with act as this was a `"push"`, we will not have
anywhere to report the status of the PipelineRuns

In this case the best way to get a report or a notification is to add it directly
with a finally task to your Pipeline or by inspecting the Repo CRD with the `tkn
pac` CLI. See the [statuses documentation](/docs/guide/statuses) which has a few
tips on how to do that.
