---
title: Metrics
weight: 16
---

# Metrics Overview

The metrics for pipelines-as-code can be accessed through the `pipelines-as-code-watcher` service on port `9090`.

pipelines-as-code supports various exporters, such as Prometheus, Google Stackdriver, and more.
You can configure these exporters by referring to the [observability configuration](../config/config-observability.yaml).

## Core PipelineRun Metrics

| Name                                                 | Type    | Description                                                        | Labels |
|------------------------------------------------------|---------|--------------------------------------------------------------------|---------|
| `pipelines_as_code_pipelinerun_count`                | Counter | Number of pipelineruns created by pipelines-as-code                | `provider`, `event-type`, `namespace`, `repository` |
| `pipelines_as_code_pipelinerun_duration_seconds_sum` | Counter | Number of seconds all pipelineruns have taken in pipelines-as-code | `namespace`, `repository`, `status`, `reason` |
| `pipelines_as_code_running_pipelineruns_count`       | Gauge   | Number of running pipelineruns in pipelines-as-code                | `namespace`, `repository` |

## Git Provider API Metrics

| Name                                                 | Type    | Description                                                        | Labels |
|------------------------------------------------------|---------|--------------------------------------------------------------------|---------|
| `pipelines_as_code_git_provider_api_request_count`   | Counter | Number of API requests submitted to git providers                  | `provider`, `event-type`, `namespace`, `repository` |

**Note:** The metric `pipelines_as_code_git_provider_api_request_count`
is emitted by both the Controller and the Watcher, since both services
use Git providers' APIs. When analyzing this metric, you may need to
combine both services' metrics. For example, using PromQL:

- `sum (pac_controller_pipelines_as_code_git_provider_api_request_count or pac_watcher_pipelines_as_code_git_provider_api_request_count)`
- `sum (rate(pac_controller_pipelines_as_code_git_provider_api_request_count[1m]) or rate(pac_watcher_pipelines_as_code_git_provider_api_request_count[1m]))`

![Prometheus query for git provider API usage metrics combined from both the Watcher and the Controller](/images/git-api-usage-metrics-prometheus-query.png)

## Queue Concurrency Metrics

The following metrics are available for monitoring the concurrency queue system that manages PipelineRun execution:

| Name                                    | Type    | Description                                                        | Labels |
|-----------------------------------------|---------|--------------------------------------------------------------------|---------|
| `pac_queue_validation_errors_total`     | Gauge   | Number of queue validation errors per repository                   | `repository`, `namespace` |
| `pac_queue_validation_warnings_total`   | Gauge   | Number of queue validation warnings per repository                 | `repository`, `namespace` |
| `pac_queue_repair_operations_total`     | Counter | Number of queue repair operations                                   | `repository`, `namespace`, `status` |
| `pac_queue_state`                       | Gauge   | Current state of concurrency queues                                | `repository`, `namespace`, `state` |
| `pac_queue_utilization_percentage`      | Gauge   | Queue utilization as percentage of concurrency limit               | `repository`, `namespace` |
| `pac_queue_recovery_duration_seconds`   | Histogram | Time taken to recover queue state                              | `repository`, `namespace` |

### Queue Metrics Details

#### Validation Metrics

- **`pac_queue_validation_errors_total`**: Tracks the number of validation errors found during periodic queue consistency checks. High values indicate queue inconsistencies that need attention.
- **`pac_queue_validation_warnings_total`**: Tracks the number of validation warnings found during periodic queue consistency checks. Warnings indicate potential issues but are less severe than errors.

#### Repair Metrics

- **`pac_queue_repair_operations_total`**: Counts the number of repair operations performed. The `status` label indicates whether the repair was `success` or `failed`.

#### State Metrics

- **`pac_queue_state`**: Shows the current state of queues with the following `state` labels:
  - `running`: Number of PipelineRuns currently executing
  - `pending`: Number of PipelineRuns waiting in the queue

#### Utilization Metrics

- **`pac_queue_utilization_percentage`**: Shows queue utilization as a percentage of the configured concurrency limit. Values close to 100% indicate high queue usage.

#### Performance Metrics

- **`pac_queue_recovery_duration_seconds`**: Measures the time taken to recover queue state during initialization or repair operations. This helps identify performance issues.

### Queue Metrics Use Cases

#### Monitoring Queue Health

```promql
# Check for repositories with validation errors
pac_queue_validation_errors_total > 0

# Monitor queue utilization
pac_queue_utilization_percentage > 80

# Track repair success rate
rate(pac_queue_repair_operations_total{status="success"}[5m]) / rate(pac_queue_repair_operations_total[5m])
```

#### Alerting Examples

```yaml
# Alert on high validation errors
- alert: QueueValidationErrors
  expr: pac_queue_validation_errors_total > 5
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High queue validation errors detected"

# Alert on high queue utilization
- alert: HighQueueUtilization
  expr: pac_queue_utilization_percentage > 90
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Queue utilization is high"
```

## Metric Collection Frequency

- **Core PipelineRun metrics**: Emitted in real-time as PipelineRuns are created and completed
- **Git Provider API metrics**: Emitted for each API request to Git providers
- **Queue metrics**: Collected every 1 minute during periodic queue validation

## Configuration

### Enabling Metrics

Metrics are enabled by default. You can configure the metrics endpoint and collection settings through the observability configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-observability
  namespace: pipelines-as-code
data:
  metrics.backend-destination: prometheus
  metrics.request-metrics-backend-destination: prometheus
  prometheus-host: 0.0.0.0
  prometheus-port: "9090"
```

### Customizing Queue Validation Frequency

The queue validation frequency can be adjusted by modifying the controller code. Currently set to run every 1 minute:

```go
ticker := time.NewTicker(1 * time.Minute) // Run every 1 minute
```

## Troubleshooting

### Common Issues

1. **High validation errors**: Indicates queue inconsistencies, often due to controller restarts or partial failures
2. **High queue utilization**: May indicate insufficient concurrency limits or high pipeline demand
3. **Long recovery times**: Suggests performance issues during queue initialization or repair

### Debugging Commands

```bash
# Start a proxy to access the metrics endpoint
kubectl proxy &

# Find the controller pod name
kubectl get pod -n pipelines-as-code | grep controller

# Access metrics via the proxy (replace POD_NAME with the actual name from above)
curl http://127.0.0.1:8001/api/v1/namespaces/pipelines-as-code/pods/POD_NAME:9090/proxy/metrics

# Filter queue metrics
curl http://127.0.0.1:8001/api/v1/namespaces/pipelines-as-code/pods/POD_NAME:9090/proxy/metrics | grep pac_queue

# Check specific repository metrics
curl http://127.0.0.1:8001/api/v1/namespaces/pipelines-as-code/pods/POD_NAME:9090/proxy/metrics | grep "repository=\"your-repo-name\""
```

## Best Practices

1. **Monitor queue validation errors**: Set up alerts for repositories with persistent validation errors
2. **Track utilization trends**: Monitor queue utilization to optimize concurrency limits
3. **Watch repair operations**: High repair rates may indicate underlying issues
4. **Set appropriate concurrency limits**: Balance resource usage with pipeline throughput
5. **Regular metric review**: Periodically review metrics to identify optimization opportunities
