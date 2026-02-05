# Log Correlation Guide

This guide explains how to trace events across the different log files in E2E test artifacts.

## Key Identifiers

### Namespace

Every E2E test creates a unique namespace with the pattern:

```
pac-e2e-ns-XXXXX
```

Where XXXXX is a random suffix. This namespace appears in all component logs and is the primary correlation key.

### Event ID

Some operations include an `event-id` in the logs that can be traced across components:

```json
{"event-id": "abc123", "msg": "processing webhook"}
```

### Repository Name

Tests often use repository names that include the test name:

```
pac-e2e-test-xxxxx
```

## Correlation Commands

### Find namespace in all logs

```bash
ns="pac-e2e-ns-xxxxx"
for log in pipelines-as-code-*.log; do
  echo "=== $log ==="
  grep "$ns" "$log" | head -20
done
```

### Trace a specific event

```bash
event_id="your-event-id"
grep "$event_id" pipelines-as-code-*.log
```

### Find errors around a timestamp

```bash
# If you know the approximate time of failure
grep "2024-01-15T10:3" pipelines-as-code-controller.log | grep -i error
```

### Correlate webhook to controller

1. Find the webhook event in webhook logs:

   ```bash
   grep "pac-e2e-ns-xxxxx" pipelines-as-code-webhook.log | grep "incoming"
   ```

2. Get the event-id from that line
3. Search controller logs:

   ```bash
   grep "event-id-here" pipelines-as-code-controller.log
   ```

## Log Format

All PAC component logs use structured JSON format:

```json
{
  "level": "info|error|debug",
  "ts": "2024-01-15T10:30:00.000Z",
  "logger": "pipelinesascode",
  "msg": "message here",
  "namespace": "pac-e2e-ns-xxxxx",
  "event-id": "optional"
}
```

### Filter by log level

```bash
# Errors only
grep '"level":"error"' pipelines-as-code-controller.log

# With namespace context
grep '"level":"error"' pipelines-as-code-controller.log | grep "pac-e2e-ns-xxxxx"
```

## Common Correlation Patterns

### Test failure → Controller error

1. From `e2e-test-output.log`, get the namespace
2. Search controller log for that namespace + error level
3. Look for the last error before test completion

### Webhook not processed

1. Check webhook log for incoming event
2. Look for validation errors
3. If no webhook log entry, check gosmee logs for delivery issues

### PipelineRun failure

1. Get PipelineRun name from test output
2. Search controller logs for that PipelineRun name
3. Check `pac-pipelineruns.yaml` for status conditions

### Timing issues

1. Note timestamp from test failure
2. Search all logs within ±30 seconds of that time
3. Look for timeout or context deadline errors

## Useful jq Commands

If the log format is clean JSON (one object per line):

```bash
# Extract all errors with namespace
cat pipelines-as-code-controller.log | jq -c 'select(.level == "error")'

# Get messages for a namespace
cat pipelines-as-code-controller.log | jq -c 'select(.namespace == "pac-e2e-ns-xxxxx") | .msg'
```
