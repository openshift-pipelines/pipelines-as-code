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

## Verify

Ensure that the pipelines-as-code controller, webhook, and watcher have come up healthy, for example:

```shell
$ kubectl get deployment -n pipelines-as-code
NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
pipelines-as-code-controller   1/1     1            1           43h
pipelines-as-code-watcher      1/1     1            1           43h
pipelines-as-code-webhook      1/1     1            1           43h
```

All three deployments should have all pods ready before moving on to ingress setup.

## Ingress

You will need a `Ingress` to point to the pipelines-as-code controller.
The ingress configuration depends on your Kubernetes provider.
Either the ingress hostname or its IP address may be used as the webhook URL.
You'll provide this configuration when connecting Pipelines as Code to your Git provider.

Here is an example working with the [nginx ingress](https://kubernetes.github.io/ingress-nginx/) controller :

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

In this example `webhook.host.tld` is the hostname that will be used for the Pipelines as Code webhook URL.

Here's an example with GKE:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    pipelines-as-code/route: controller
  name: pipelines-as-code
  namespace: pipelines-as-code
  annotations:
    kubernetes.io/ingress.class: gce
spec:
  defaultBackend:
    service:
      name: pipelines-as-code-controller
      port:
        number: 8080
```

In this example, you will use the ingress's IP address rather than a hostname for the webhook URL.
You can find the ingress's address via `kubectl get ingress pipelines-as-code -n pipelines-as-code`.

If you can't or don't want to set up an ingress on your cluster (for example, you're using a kind cluster,
or you just want to experiment with Pipelines as Code before setting up an ingress),
see [Proxy service for PAC controller](./installation.md#proxy-service-for-pac-controller).

## Tekton Dashboard integration

If you have [Tekton Dashboard](https://github.com/tektoncd/dashboard). You can
just add the key `tekton-dashboard-url` in the `pipelines-as-code` config map set
to the full URL of the `Ingress` host to get tekton dashboard logs URL.
