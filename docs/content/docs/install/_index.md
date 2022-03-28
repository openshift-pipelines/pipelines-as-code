---
title: Installation
weight: 1
---
# Pipelines as Code - Installation

Pipelines-as-Code support different installation method to Git provider
platforms (i.e: GitHub, Bitbucket etc..)

The preferred method to use Pipelines-as-Code is configured with a [GitHub
Application](https://docs.github.com/en/developers/apps/getting-started-with-apps/about-apps).

## Install Pipelines-as-Code

In order to install and use Pipelines-as-Code, you need to

* Install the Pipelines-as-Code infrastructure on your cluster
* Configure your Git Provider (eg: a GitHub Application) to access Pipelines as Code.

### Install Pipelines as Code infrastructure

To install Pipelines as Code on your cluster you simply need to run this command
:

```shell
VERSION=0.5.3
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.yaml
```

If you would like to install the current development version you can simply
install it like this :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml
```

This will apply the `release.yaml` to your kubernetes cluster, creating the admin
namespace `pipelines-as-code`, the roles and all other bits needed.

The `pipelines-as-code` namespace is where the Pipelines-as-Code infrastructure
runs and is supposed to be accessible only by the admins.

The Route URL for the Pipelines as Code Controller is automatically created when
you apply the release.yaml. You will need to reference this url when configuring
your github provider.

You can run this command to get the route created on your cluster:

```shell
echo https://$(oc get route -n pipelines-as-code el-pipelines-as-code-interceptor -o jsonpath='{.spec.host}')
```

### RBAC

Non `system:admin` users needs to be allowed explicitly to create repositories
CRD in their namespace

To allow them you need to create a `RoleBinding` on the namespace to the
`openshift-pipeline-as-code-clusterrole`.

For example assuming we want `user` being able to create repository CRD in the
namespace `user-ci`, if we use the openshift `oc` cli :

```shell
oc adm policy add-role-to-user openshift-pipeline-as-code-clusterrole user -n user-ci
```

or via kubectl applying this yaml :

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: openshift-pipeline-as-code-clusterrole
  namespace: user-ci
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: openshift-pipeline-as-code-clusterrole
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: user
```

## Provider Setup

After installing Pipelines as Code you are now ready to configure your Git
provider. Choose your preferred install method, if you don't have any
preferences the preferred install method is the Github Application method.

* [Github Application](./github_apps).
* [Github Webhook](./github_webhook)
* [Gitlab](./gitlab)
* [Bitbucket Server](./bitbucket_server)
* [Bitbucket Cloud](./bitbucket_cloud)

## Pipelines-As-Code configuration settings

There is a few things you can configure via the configmap `pipelines-as-code` in
the `pipelines-as-code` namespace.

* `application-name`

  The name of the application showing for example in the GitHub Checks
  labels. Default to `"Pipelines as Code CI"`

* `max-keep-days`

  The number of the day to keep the PipelineRuns runs in the `pipelines-as-code`
  namespace. We install by default a cronjob that cleans up the PipelineRuns
  generated on events in pipelines-as-code namespace. Note that these
  PipelineRuns are internal to Pipelines-as-code are separate from the
  PipelineRuns that exist in the user's GitHub repository. The cronjob runs
  every hour and by default cleanups PipelineRuns over a day. This configmap
  setting doesn't affect the cleanups of the user's PipelineRuns which are
  controlled by the [annotations on the PipelineRun definition in the user's
  GitHub repository](#pipelineruns-cleanups).

* `secret-auto-create`

  Whether to auto create a secret with the token generated via the Github
  application to be used with private repositories. This feature is enabled by
  default.

* `remote-tasks`

  Let allows remote tasks from pipelinerun annotations. This feature is enabled by
  default.

* `hub-url`

  The base url for the [tekton hub](https://github.com/tektoncd/hub/)
  API. default to the [public hub](https://hub.tekton.dev/):

  <https://api.hub.tekton.dev/v1>

## Kubernetes

Pipelines as Code should work directly on kubernetes/minikube/kind. You just need to install the release.yaml
for [pipeline](https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml)
, [triggers](https://storage.googleapis.com/tekton-releases/triggers/latest/release.yaml) and
its [interceptors](https://storage.googleapis.com/tekton-releases/triggers/latest/interceptors.yaml) on your cluster.
The release yaml to install pipelines are for the released version :

```shell
VERSION=0.5.3
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release-$VERSION.k8s.yaml
```

and for the nightly :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/release-$VERSION/release.k8s.yaml
```

If you have [Tekton Dashboard](https://github.com/tektoncd/dashboard). You can
just add the key `tekton-dashboard-url` in the `pipelines-as-code` configmap
set to the full url of the `Ingress` host to get tekton dashboard logs url.

## CLI

`Pipelines as Code` provide a CLI which is designed to work as tkn plugin. To
install the plugin follow the instruction from the [CLI](./guide/cli)
documentation.
