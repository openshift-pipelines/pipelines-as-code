---
title: Custom certificates
weight: 4
---
# Custom certificates

If you need to configure Pipelines-as-Code with a Git repository that
requires a privately signed or custom certificate to access, then you will
need to expose the certificate to Pipelines-as-Code.

## OpenShift

If you have installed Pipelines-as-Code through the OpenShift Pipelines
operator, then you will need to [add your custom certificate to the cluster via
the Proxy object.](https://docs.openshift.com/container-platform/4.11/networking/configuring-a-custom-pki.html#nw-proxy-configure-object_configuring-a-custom-pki)
The operator will expose the certificate in all OpenShift Pipelines
components and workloads, including Pipelines-as-Code.

## Kubernetes

### Create a ConfigMap containing the certificate

```shell
kubectl -n pipelines-as-code create configmap git-repo-cert --from-file=git.crt=<path to ca.crt>
```

### Mount the ConfigMap in the pods

Follow [this guide](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#add-configmap-data-to-a-volume)
to mount the ConfigMap in the `pipelines-as-code-controller` and
`pipelines-as-code-watcher` Deployments in the cluster in the
`pipelines-as-code` namespace.

### Include `mountPath` in `SSL_CERT_DIR`

Say, you mounted the ConfigMap with the `mountPath` as `/pac-custom-certs`.
To include this directory in the paths where the certificates are looked up,
set the environment variable `SSL_CERT_DIR` in the relevant Pipelines-as-Code
Deployments.

```shell
kubectl set env deployment pipelines-as-code-controller pipelines-as-code-watcher -n pipelines-as-code SSL_CERT_DIR=/pac-custom-certs:/etc/ssl/certs:/etc/pki/tls/certs:/system/etc/security/cacerts
```

Pipelines-as-Code should now be able to access the repository using the
custom certificate.
