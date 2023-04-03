---
title: Installation Through Operator
weight: 2.1
---
# Installation Through Operator

The easiest way to install Pipelines as Code on OpenShift is with the [Red Hat Openshift Pipelines Operator](https://docs.openshift.com/container-platform/latest/cicd/pipelines/installing-pipelines.html).

On the OpenShift Pipelines Operator, the default namespace is `openshift-pipelines`.

**Note:**

When Pipelines as Code is installed through the [Tekton Operator](https://github.com/tektoncd/operator) the configurations of Pipelines as Code is
controlled by [TektonConfig Custom Resource](https://github.com/tektoncd/operator/blob/main/docs/TektonConfig.md) rather then `pipeline-as-code` configmap.
That means Tekton Operator will revert back the configurations changes done directly on `pipeline-as-code` configmap.

The default configurations for Pipelines as Code in `TektonConfig` looks like below

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
          application-name: Pipelines as Code CI
          auto-configure-new-github-repo: 'false'
          error-log-snippet: 'true'
          error-detection-from-container-logs: 'false'
          hub-url: 'https://api.hub.tekton.dev/v1'
          hub-catalog-name: tekton
          error-detection-max-number-of-lines: '50'
          error-detection-simple-regexp: >-
            ^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([
            ]*)?(?P<error>.*)
          secret-auto-create: 'true'
          secret-github-app-token-scoped: 'true'
```

The supported configurations for Pipelines as Code can be added/updated under `settings`

**Note:**

By default, Tekton Operator installs Pipelines as Code.

```yaml
spec:
  platforms:
    openshift:
      pipelinesAsCode:
        enable: true
        settings:
          
```

To disable default installation use can provide `enable: false` like below

```yaml
spec:
  platforms:
    openshift:
      pipelinesAsCode:
        enable: false
        settings:
          
```
