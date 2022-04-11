---
title: Kubernetes
weight: 20
---
# Kubernetes

Pipelines as Code works on kubernetes/minikube/kind.

## Prerequisites

You will need to pre-install the [pipeline](https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml) `release.yaml`
file on your kubernetes cluster.

## Install

The release YAML to install pipelines are for the released version :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/stable/release.k8s.yaml
```

and for the nightly :

```shell
kubectl apply -f https://raw.githubusercontent.com/openshift-pipelines/pipelines-as-code/nightly/release.k8s.yaml
```

## Ingress

You will need a `Ingress` to point to the pipelines-as-code controller, here is an example working with the [nginx ingress](https://kubernetes.github.io/ingress-nginx/) controller :

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    pipelines-as-code/route: controller
  name: pipelines-as-code
  namespace: pipelines-as-code
spec:
  ingressClassName: nginx
  rules:
  - host: webhook.host.tld
    http:
      paths:
      - backend:
          service:
            name: pipelines-as-code-controller
            port:
              number: 8080
        path: /
        pathType: Prefix
```

In this example `webhook.host.tld` is the hostname for your pipeline's controller to fill as the webhook URL in the provider platform setup.

## Tekton Dashboard integration

If you have [Tekton Dashboard](https://github.com/tektoncd/dashboard). You can
just add the key `tekton-dashboard-url` in the `pipelines-as-code` configmap set
to the full URL of the `Ingress` host to get tekton dashboard logs URL.
