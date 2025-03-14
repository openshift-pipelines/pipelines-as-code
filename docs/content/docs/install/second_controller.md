---
title: Multiple GitHub Applications
---

# Multiple GitHub application support

{{< tech_preview "Multiple GitHub apps support" >}}

Pipelines-as-Code supports running multiple GitHub applications on the same
cluster. This allows you to have multiple GitHub applications pointing to the
same cluster from different installation (like public GitHub and GitHub
Enterprise).

## Running a second controller with a different GitHub application

Each new install for different GitHub applications have their own controller
with a Service and a
[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) or
a [OpenShift Route](https://docs.openshift.com/container-platform/latest/networking/routes/route-configuration.html)
attached to it.

Each controller can have their own [Configmap]({{< relref "/docs/install/settings" >}}) for their configuration and should have their own
secret with the GitHub application `private key`/`application_id` and
`webhook_secret`. See the documentation on how to configure those secrets
[here]({{< relref "/docs/install/github_apps#manual-setup" >}}).

The controller have three different environment variable on its container to
drive this:

| Environment Variable       | Description                                              | Example Value   |
|----------------------------|----------------------------------------------------------|-----------------|
| `PAC_CONTROLLER_LABEL`     | A unique label to identify this controller               | `ghe`           |
| `PAC_CONTROLLER_SECRET`    | The Kubernetes secret with the GitHub application secret | `ghe-secret`    |
| `PAC_CONTROLLER_CONFIGMAP` | The Configmap with the Pipelines-as-Code config          | `ghe-configmap` |

{{< hint info >}}
While you need multiple controllers for different GitHub applications, only one
`watcher` (the Pipelines-as-Code reconciler that reconcile the status on the
GitHub interface) is needed.
{{< /hint >}}

## Script to help running a second controller

We have a script in our source code repository to help deploying a second
controller with its associated service and ConfigMap. As well setting the
environment variables.

Its located in the `./hack` directory and called [second-controller.py](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/hack/second-controller.py)

To use it first check-out the Pipelines-as-Code repository:

```shell
git clone https://github.com/openshift-pipelines/pipelines-as-code
```

You need to make sure the python-yaml module is installed, you can install it by
multiple ways (i.e: your operating system package manager) or simply can use pip:

```shell
python3 -mpip install PyYAML
```

And run it with:

```shell
python3 ./hack/second-controller.py LABEL
```

This will output the generated yaml on the standard output, if you are happy
with the output you can apply it on your cluster with `kubectl`:

```shell
python3 ./hack/second-controller.py LABEL|kubectl apply -f -
```

There is multiple flags you can use to fine grain the output of this script, use
the `--help` flag to list all the flags that can be passed to the script.
