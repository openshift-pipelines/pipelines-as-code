---
title: Infrastructure install
weight: 2
---
# Manual install

To install Pipelines as Code on your cluster you simply need to run this command
:

```shell
VERSION=0.5.5
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/stable/release.yaml
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
echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
```

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

## CLI

`Pipelines as Code` provide a CLI which is designed to work as tkn plugin. To
install the plugin follow the instruction from the [CLI](./guide/cli)
documentation.
