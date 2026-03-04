---
title: "Configuration"
weight: 1
---

This page covers the operational tasks for managing Pipelines-as-Code configuration:
viewing, editing, and applying changes to the `pipelines-as-code` ConfigMap.

For an overview of all configuration layers (global ConfigMap and per-repository CR),
see [Settings]({{< relref "settings" >}}).

To view the current configuration:

```bash
kubectl get configmap pipelines-as-code -n pipelines-as-code -o yaml
```

For the complete reference of all configuration fields, see the [ConfigMap Reference]({{< relref "/docs/api/configmap" >}}).

## Applying Configuration Changes

To update the configuration:

```bash
kubectl edit configmap pipelines-as-code -n pipelines-as-code
```

Or apply changes from a file:

```bash
kubectl apply -f pipelines-as-code-config.yaml
```

Most configuration changes take effect immediately. Some settings may require a controller restart:

```bash
kubectl rollout restart deployment/pipelines-as-code-controller -n pipelines-as-code
```

## See Also

- [Global Repository Settings]({{< relref "global-repository-settings" >}}) - Configure default settings for all repositories
- [Logging Configuration]({{< relref "logging" >}}) - Configure log levels and debugging
- [Metrics]({{< relref "metrics" >}}) - Monitor Pipelines-as-Code with Prometheus
