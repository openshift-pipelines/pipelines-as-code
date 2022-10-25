---
title: Installation
weight: 2
---
# Installation

## Operator Install

The easiest way to install Pipelines as Code on OpenShift is with the [Red Hat Openshift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/installing-pipelines.html).

On the Openshift Pipelines Operator, the default namespace is `openshift-pipelines`.

## Manual Install

### Prerequisite

Before installing Pipelines As Code, please verify [tektoncd/pipeline](https://github.com/tektoncd/pipeline) is installed. You can install the latest released version using the following command

```shell
  kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
```

{{< hint info >}}
If you are not installing the latest version, ensure you are running Tekton Pipeline greater than v0.25.0_
{{< /hint >}}

If you want to do a manual installation of the stable release of Pipelines as Code
on your OpenShift cluster you can apply the template with kubectl :

```shell
# OpenShift
kubectl patch tektonconfig config --type="merge" -p '{"spec": {"addon":{"enablePipelinesAsCode": false}}}'
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

On Openshift the Route URL for the Pipelines as Code Controller is automatically created when
you apply the `release.yaml`. You will need to reference this URL when configuring
your GitHub provider.

You can run this command to get the route created on your cluster:

```shell
echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
```

### Kubernetes

Kubernetes installation is a bit more involved head over [here](/docs/install/kubernetes) for more details.

## RBAC

Non `system:admin` users needs to be allowed explicitly to create repositories
CRD in their namespace

To allow them you need to create a `RoleBinding` on the namespace to the
`openshift-pipeline-as-code-clusterrole`.

For example assuming we want `user` being able to create repository CRD in the
namespace `user-ci`, if we use the openshift `oc` cli :

```shell
oc adm policy add-role-to-user openshift-pipeline-as-code-clusterrole user -n user-ci
```

or through kubectl applying this YAML :

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

`Pipelines as Code` provide a CLI which is designed to work as tkn plug-in. To
install the plug-in follow the instruction from the [CLI](/docs/guide/cli)
documentation.

## Controller TLS Setup

Pipelines As Code Controller now support both `HTTP` and `HTTPS`. Usually, you configure the TLS directly on the
ingress/Route pointing to the controller. If you want to configure the TLS directly on the controller you can do so
by following this guide.

First, create a secret which includes those certificates

```shell
  kubectl create secret generic -n pipelines-as-code pipelines-as-code-tls-secret \
    --from-file=cert=/path/to/crt/file \
    --from-file=key=/path/to/key/file
```

You can now restart the `pipelines-as-code-controller` pod in `pipelines-as-code` namespace and by the time the controller will be
restarted it will use the tls secrets.

NOTE:

- It is required to create the secret named `pipelines-as-code-tls-secret`, or you will have to update the secret name in
controller deployment.
- If you have different keys in your secret other than `cert` and `key`, you will need to update controller deployment envs
and subsequently apply this changes on upgrade (for example through [kustomize](https://kustomize.io/) or other methods)

You can use following command to update the envs on the controller

```shell
  kubectl set env deployment pipelines-as-code-controller -n pipelines-as-code TLS_KEY=<key> TLS_CERT=<cert>
```

## Proxy service for PAC controller

### What

proxy service is used to forward request coming to `pipelines-as-code-controller` service

### Why

PAC requires externally accessible URL to configure for GitHub, GitLab, BitBucket and there are few clusters which doesn't expose services to external world ex: Minikube, Kind and so on.

### Proposed solution

To handle such scenario for minikube/kind cluster lets use [smee.io](https://smee.io/)

### Steps to configure smee.io

- Generate your own URL by going to [smee.io/new](https://smee.io/new)
- Copy `Webhook Proxy URL`
- Add `Webhook Proxy URL` URL in container args of `deployment.yaml`.

Ex: `'<replace Webhook Proxy URL>'` -> `'https://smee.io/oLHu7IjUV4wGm2tJ'`

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

- Execute

```yaml
kubectl create -f deployment.yaml -n pipelines-as-code
```

- Use `Webhook Proxy URL` to configure in GitHub, GitLab and BitBucket.

Basically use `Webhook Proxy URL` in all the places wherever `pipelines-as-code-controller` service URL used.
