---
title: Kubernetes
weight: 20
---
# Kubernetes

Pipelines-as-Code works on kubernetes/minikube/kind.

## Prerequisites

You will need to pre-install the
[pipeline](https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml)
`release.yaml` file on your kubernetes cluster.

You will need at least a kubernetes version greater than 1.23

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

You will need a
[`Ingress`](https://kubernetes.io/docs/concepts/services-networking/ingress/)
setup to point to make your pipelines-as-code controller available to `Github`,
`GitLab` or other `Git` providers.

The ingress configuration depends on your Kubernetes provider. See below for
some examples.

Either the ingress hostname or its IP address may be used as the webhook URL.
You'll have to provide this URL when connecting Pipelines-as-Code to
your Git provider. You can find the ingress's address via
`kubectl get ingress pipelines-as-code -n pipelines-as-code`.

If you are quickly trying pipelines-as-code and do not want to setup the Ingress
access, the `tkn pac bootstrap` [cli](../../guide/cli) command will let you
set-up a [gosmee](https://github.com/chmouel/gosmee) deployment using the
webhook URL remote forwarder `https://hook.pipelinesascode.com`.

### [GKE](https://cloud.google.com/kubernetes-engine)

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

### [Nginx Ingress Controller](https://kubernetes.github.io/ingress-nginx/)

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

In this example `webhook.host.tld` is the hostname for your pipeline's
controller to fill as the webhook URL in the Git provider.

## Tekton Dashboard integration

If you have [Tekton Dashboard](https://github.com/tektoncd/dashboard). You can
just add the key `tekton-dashboard-url` in the `pipelines-as-code` config map
set to the full URL of the `Ingress` host to get tekton dashboard logs URL.
