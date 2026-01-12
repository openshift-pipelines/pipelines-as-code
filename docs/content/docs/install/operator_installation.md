---
title: Installation Through Operator
weight: 2.1
---
# Installation Through Operator

The easiest way to install Pipelines-as-Code on OpenShift is with the [Red Hat OpenShift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/installing-pipelines.html).

On the OpenShift Pipelines Operator, the default namespace is `openshift-pipelines`.

**Note:**

When Pipelines-as-Code is installed through the [Tekton Operator](https://github.com/tektoncd/operator) the configuration of Pipelines-as-Code is
controlled by [TektonConfig Custom Resource](https://github.com/tektoncd/operator/blob/main/docs/TektonConfig.md#openshiftpipelinesascode).
That means Tekton Operator will revert the configuration changes done directly
on `pipelines-as-code` ConfigMap or `OpenShiftPipelinesAsCode` custom resource.

The default configurations for Pipelines-as-Code in `TektonConfig` looks like
below.
Note that since version v0.37.0, Pipelines-as-Code defaults to using Artifact
Hub. The public Tekton Hub (hub.tekton.dev) has been deprecated and is no longer
available. You can still use custom self-hosted Tekton Hub instances by
configuring them as custom catalogs (see [Remote Hub Catalogs](/docs/install/settings#remote-hub-catalogs)).

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  platforms:
    openshift:
      pipelinesAsCode:
        enable: true
        settings:
          bitbucket-cloud-check-source-ip: 'true'
          remote-tasks: 'true'
          application-name: Pipelines-as-Code CI
          auto-configure-new-github-repo: 'false'
          error-log-snippet: 'true'
          error-detection-from-container-logs: 'false'
          enable-cancel-in-progress-on-pull-requests: 'false'
          enable-cancel-in-progress-on-push: 'false'
          skip-push-event-for-pr-commits: 'true'
          hub-url: 'https://artifacthub.io'
          hub-catalog-type: 'artifacthub'
          error-detection-max-number-of-lines: '50'
          error-detection-simple-regexp: >-
            ^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([
            ]*)?(?P<error>.*)
          secret-auto-create: 'true'
          secret-github-app-token-scoped: 'true'
          remember-ok-to-test: 'true'
```

You can add or update all supported configuration keys for Pipelines-as-Code under `settings`. After you change the `TektonConfig` custom resource, the operator updates the configuration of your `pipelines-as-code` ConfigMap automatically.

**Note:**

By default, Tekton Operator installs Pipelines-as-Code, default value of `enable` is `true` as in the following example:

```yaml
spec:
  platforms:
    openshift:
      pipelinesAsCode:
        enable: true
        settings:

```

To disable installation of Pipelines-as-Code, you can set `enable: false` as in the following example:

```yaml
spec:
  platforms:
    openshift:
      pipelinesAsCode:
        enable: false
        settings:

```
