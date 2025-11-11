---
title: Manual Installation
weight: 2
---
# Installation

## Operator Install

Follow [Operator Installation](./operator_installation.md) to install Pipelines-as-Code on OpenShift.

## Manual Install

### Prerequisite

Before installing Pipelines-as-Code, please verify that
[tektoncd/pipeline](https://github.com/tektoncd/pipeline) is installed. You can
install the latest released version using the following command:

```shell
  kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
```

{{< hint info >}}
If you are not installing the most recent version, ensure that you have Tekton Pipeline installed and running at a version higher than v0.44.0.
{{< /hint >}}

If you want to do a manual installation of the stable release of Pipelines-as-Code
on your OpenShift cluster you can apply the template with kubectl :

```shell
# OpenShift
kubectl patch tektonconfig config --type="merge" -p '{"spec": {"platforms": {"openshift":{"pipelinesAsCode": {"enable": false}}}}}'
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/stable/release.yaml

# Kubernetes
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/stable/release.k8s.yaml
```

If you want to install the current development version you can simply
install it like this :

```shell
# OpenShift
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.yaml

# Kubernetes
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.k8s.yaml
```

This will apply the `release.yaml` to your OpenShift cluster, creating the admin
namespace `pipelines-as-code`, the roles and all other bits needed.

The `pipelines-as-code` namespace is where the Pipelines-as-Code infrastructure
runs and is supposed to be accessible only by the admins.

### OpenShift

On OpenShift the Route URL for the Pipelines-as-Code Controller is automatically created when
you apply the `release.yaml`. You will need to reference this URL when configuring
your GitHub provider.

You can run this command to get the route created on your cluster:

```shell
echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
```

### Kubernetes

Kubernetes installation is a bit more involved head over [here](/docs/install/kubernetes) for more details.

## RBAC

Non-`system:admin` users need to be explicitly allowed to create Repository
CRDs in their namespace.

To allow them, you need to create a `RoleBinding` on the namespace to the
`openshift-pipeline-as-code-clusterrole`.

For example, assuming we want `user` to be able to create Repository CRDs in the
namespace `user-ci`, if we use the OpenShift `oc` CLI:

```shell
oc adm policy add-role-to-user openshift-pipeline-as-code-clusterrole user -n user-ci
```

or through kubectl by applying this YAML:

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

## CLI

Pipelines-as-Code provides a CLI which is designed to work as a tkn plug-in. To
install the plug-in, follow the instructions from the [CLI](/docs/guide/cli)
documentation.

## Controller TLS Setup

The Pipelines-as-Code Controller now supports both `HTTP` and `HTTPS`. Usually, you configure the TLS directly on the
ingress/Route pointing to the controller. If you want to configure the TLS directly on the controller, you can do so
by following this guide.

First, create a secret which includes these certificates:

```shell
  kubectl create secret generic -n pipelines-as-code pipelines-as-code-tls-secret \
    --from-file=cert=/path/to/crt/file \
    --from-file=key=/path/to/key/file
```

You can now restart the `pipelines-as-code-controller` pod in the `pipelines-as-code` namespace and when the controller is
restarted, it will use the TLS secrets.

NOTE:

- It is required to create the secret named `pipelines-as-code-tls-secret`, or you will have to update the secret name in the
controller deployment.
- If you have different keys in your secret other than `cert` and `key`, you will need to update the controller deployment environment variables
and subsequently apply these changes on upgrade (for example through [kustomize](https://kustomize.io/) or other methods).

You can use the following command to update the environment variables on the controller:

```shell
  kubectl set env deployment pipelines-as-code-controller -n pipelines-as-code TLS_KEY=<key> TLS_CERT=<cert>
```

## Proxy Service for PAC Controller

Pipelines-as-Code requires an externally accessible URL to receive events from
Git providers. If you're developing locally (such as on kind or Minikube) or
cannot set up an ingress on your cluster, you can also use a proxy service to
expose the `pipelines-as-code-controller` service and allow it to receive
events.

This is useful for testing and development purposes, but not recommended for
production since gosmee and the platform running
<https://hook.pipelinesascode.com>
have no support or security guarantees.

### Proxying with hook.pipelinesascode.com

To handle this scenario for minikube/kind cluster, let's use [hook.pipelinesascode.com](https://hook.pipelinesascode.com/)

- Generate your own URL by going to [hook.pipelinesascode.com/new](https://hook.pipelinesascode.com/new)
- Copy the `Webhook Proxy URL`
- Add the `Webhook Proxy URL` in the container args of `deployment.yaml`.

ex: `'<replace Webhook Proxy URL>'` -> `'https://hook.pipelinesascode.com/oLHu7IjUV4wGm2tJ'`

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: gosmee-client
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gosmee-client
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: gosmee-client
    spec:
      containers:
        - name: gosmee-client
          image: 'ghcr.io/chmouel/gosmee:main'
          args:
            - '<replace Webhook Proxy URL>'
            - $(SVC)
          env:
            - name: SVC
              value: >-
                http://pipelines-as-code-controller.pipelines-as-code.svc.cluster.local:8080
      restartPolicy: Always
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%
  revisionHistoryLimit: 10
  progressDeadlineSeconds: 600
```

- Execute:

```yaml
kubectl create -f deployment.yaml -n pipelines-as-code
```

- Use the `Webhook Proxy URL` to configure in GitHub, GitLab and Bitbucket.

Basically, use the `Webhook Proxy URL` in all places wherever the `pipelines-as-code-controller` service URL is used.
