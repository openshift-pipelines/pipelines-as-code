---
title: Global Repository settings
weight: 4
---
{{< tech_preview "Global repository settings" >}}

## Pipelines-as-Code global repository settings

Pipelines-as-Code let you have a global repository for all your Repo settings.
This allows you to define settings that will be applied to all repositories on
your cluster.

The global repository setting are set as a fallback for all repositories, if
the local repository on the namespace don't override it.

The global repository have to be created in the namespace where the
`pipelines-as-code` controller is installed (usually `pipelines-as-code` or
`openshift-pipelines`).

The global repository CR should not have a `spec.url` defined.

By default the repository needs to be named `pipelines-as-code` but you can
redefine it by defining the environment variable
`PAC_CONTROLLER_GLOBAL_REPOSITORY` on the controller Deployment.

The settings that can be defined in the global repository are:

- [Concurrency Limit]({{< relref "/docs/guide/repositorycrd.md#concurrency" >}}).
- [PipelineRun Provenance]({{< relref "/docs/guide/repositorycrd.md#pipelinerun-definition-provenance" >}}).
- [Repository Policy]({{< relref "/docs/guide/policy" >}}).
- [Repository GitHub App Token scope.]({{< relref "/docs/guide/repositorycrd.md#scoping-the-github-token-using-global-configuration" >}}).
- The git provider auth settings like user, token, url etc...
  The `type` needs to be defined in the namespace repository settings and need to match the `type` of the global repository (see below for an example).
- [Custom Parameters]({{< relref "/docs/guide/customparams.md" >}}).
- [The incoming webhooks rules]({{< relref "/docs/guide/incoming_webhook.md" >}}).

Note that the custom parameters and the incoming rules don't get merged with the
namespace repository settings, they are only used if none is defined in the namespace.

{{< hint info >}}
global settings only gets applied at "runtime", they are not used by the tkn pac create repo command.
{{< /hint >}}

### Example of how the global repository settings are applied

- if you have a Repository CR in the user namespace

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

- and a have a global Repository CR on the controller namespace:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: pipelines-as-code
  namespace: pipelines-as-code
spec:
  concurrency_limit: 1
  params:
    - name: custom
      value: "value"

  git_provider:
    type: gitlab
    secret:
      name: "gitlab-token"
    webhook_secret:
      name: gitlba-webhook-secret
```

On this example the Repository `repo` will have a concurrency limit of 2 since
the setting comes from the user namespace and ignored from the global repository. The
parameter `custom` will be set to `value` and ready to be used to every
repository that don't define any other custom parameters.

Since the local repository CR has the git_provider.type `gitlab` like the
global repository CR the git provider settings for the
[GitLab]({{< relref "/docs/install/gitlab.md#create-a-repository-and-configure-webhook-manually" >}})
will be taken from the global repository. The secret referenced will be fetched
from where the global repository is defined.

### Types when git provider settings gets applied

These are the types you can setup for the git provider settings. Those are
only used when doing incoming webhooks or global repository settings. They are
only used for webhook based git providers (i.e: everything except GitHub Apps
installation), in this case the type github means repository configured using
[github webhooks]({{< relref "/docs/install/github_webhook.md" >}}):

- github
- gitlab
- gitea
- bitbucket-cloud
- bitbucket-server

The global repository settings for git provider can currently only reference one
type of provider on a cluster. The user would need to specify their own provider
info in their own Repository CR if they don't want to use the global settings or
want to target another repository.
