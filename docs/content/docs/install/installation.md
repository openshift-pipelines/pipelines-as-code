---
title: Installation
weight: 2
---
# Installation

## Operator Install

The easiest way to install Pipelines as Code on OpenShift is with the [Red Hat Openshift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/installing-pipelines.html).

On the Openshift Pipelines Operator the default namespace is `openshift-pipelines`.

## Manual Install

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
