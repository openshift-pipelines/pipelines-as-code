---
title: Global Repository Settings
weight: 4
---
{{< tech_preview "Global repository settings" >}}

## Pipelines-as-Code Global Repository Settings

Pipelines-as-Code lets you have a global repository for settings of all your
local repositories. This enables you to define settings that will be applied to
all local repositories on your cluster.

The global repository settings serve as a fallback for all repositories if the
local repository settings in the namespace do not override them.

The global repository must be created in the namespace where the
`pipelines-as-code` controller is installed (usually `pipelines-as-code` or
`openshift-pipelines`).

The global repository Custom Resource (CR) does not need a `spec.url` field. The
field can either be blank or point to an unknown destination, such as:

<https://pac.global.repo>

By default, the global repository should be named `pipelines-as-code` unless
you redefine it by setting the environment variable
`PAC_CONTROLLER_GLOBAL_REPOSITORY` in the controller and watcher Deployment.

The settings that can be defined in the global repository are:

- [Concurrency Limit]({{< relref "/docs/guide/repositorycrd.md#concurrency" >}}).
- [PipelineRun Provenance]({{< relref "/docs/guide/repositorycrd.md#pipelinerun-definition-provenance" >}}).
- [Repository Policy]({{< relref "/docs/guide/policy" >}}).
- [Repository GitHub App Token Scope]({{< relref "/docs/guide/repositorycrd.md#scoping-the-github-token-using-global-configuration" >}}).
- Git provider auth settings such as user, token, URL, etc.
  - The `type` must be defined in the namespace repository settings and must match the `type` of the global repository (see below for an example).
- [Custom Parameters]({{< relref "/docs/guide/customparams.md" >}}).
- [Incoming Webhooks Rules]({{< relref "/docs/guide/incoming_webhook.md" >}}).

{{< hint info >}}
Global settings are only applied when running via a Git provider event; they are not applied when for example using the `tkn pac` cli.
{{< /hint >}}

### Example of How Global Repository Settings Are Applied

- If you have a Repository CR in the namespace named `user-namespace`:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: repo
  namespace: user-namespace
spec:
  url: "https://my.git.com"
  concurrency_limit: 2
  git_provider:
    type: gitlab
```

- And a global Repository CR in the namespace where the controller and the watcher is located:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: pipelines-as-code
  namespace: pipelines-as-code
spec:
  url: "https://paac.repo"
  concurrency_limit: 1
  params:
    - name: custom
      value: "value"
  git_provider:
    type: gitlab
    secret:
      name: "gitlab-token"
    webhook_secret:
      name: gitlab-webhook-secret
```

In this example, the Repository `repo` will have a concurrency limit of 2 since
the setting comes from the user namespace and is ignored from the global
repository. The parameter `custom` will be set to `value` and will be available
for every repository that does not define other custom parameters.

Since the local Repository CR has the `git_provider.type` set to `gitlab`, like
the global Repository CR, the Git provider settings for [GitLab]({{< relref "/docs/install/gitlab.md#create-a-repository-and-configure-webhook-manually" >}})
will be taken from the global repository. The secret referenced will be fetched
from where the global repository is defined.

### Webhook Based provider global settings

These are the `spec.git_provider.type` you can set up for the Git provider
settings. They are only used when handling incoming webhooks or global
repository settings. They are used for webhook-based Git providers (i.e.,
everything except GitHub Apps installations). In this case, the type `github`
means a repository configured using [GitHub webhooks]({{< relref "/docs/install/github_webhook.md" >}}):

- github
- gitlab
- gitea
- bitbucket-cloud
- bitbucket-datacenter

The global repository settings for the Git provider can currently only
reference one type of provider on a cluster. The user would need to specify
their own provider information in their own Repository CR if they do not want
to use the global settings or want to target another provider.
