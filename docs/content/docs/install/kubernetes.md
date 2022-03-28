---
title: Kubernetes
weight: 20
---
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

