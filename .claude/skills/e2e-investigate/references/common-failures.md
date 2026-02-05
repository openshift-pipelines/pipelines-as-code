# Common E2E Test Failure Patterns

This document catalogs common failure patterns seen in E2E tests and how to identify them.

## Rate Limiting

### Symptoms

- Test timeouts after waiting for webhook/status
- Controller logs show 403 or 429 errors
- `api-instrumentation/` JSON shows rate limit responses

### Log Pattern

```
"msg": "rate limit exceeded"
"status": 403
"x-ratelimit-remaining": "0"
```

### Investigation

```bash
grep -i "rate.limit\|403\|429" pipelines-as-code-controller.log
cat api-instrumentation/*.json | grep -i ratelimit
```

### Common Causes

- Too many API calls in quick succession
- Token with insufficient permissions
- GitHub secondary rate limits

## Webhook Delivery Failures

### Symptoms

- Test hangs waiting for webhook
- No entries in controller log for the namespace
- gosmee shows delivery errors

### Log Pattern (gosmee)

```
"error": "connection refused"
"status": 502
"msg": "failed to forward webhook"
```

### Investigation

```bash
grep -i "error\|failed" gosmee/main.log
# Check if webhook was received at all
grep "pac-e2e-ns-xxxxx" pipelines-as-code-webhook.log
```

### Common Causes

- Webhook endpoint not ready
- Network issues in test cluster
- gosmee relay problems

## PipelineRun Timeouts

### Symptoms

- Test timeout with "waiting for PipelineRun"
- PipelineRun stuck in Running state
- No completion event

### Investigation

```bash
# Check PipelineRun status
grep "pac-e2e-ns-xxxxx" pac-pipelineruns.yaml | head -50

# Look for timeout in watcher
grep "timeout\|deadline" pipelines-as-code-watcher.log
```

### Common Causes

- Slow image pulls
- Resource constraints
- Task failures within pipeline

## Authentication Failures

### Symptoms

- 401 errors in logs
- "bad credentials" messages
- Token refresh failures

### Log Pattern

```
"msg": "authentication failed"
"status": 401
"error": "bad credentials"
```

### Investigation

```bash
grep -i "401\|auth\|credential\|token" pipelines-as-code-controller.log
```

### Common Causes

- Expired tokens
- Incorrect secret configuration
- Revoked app installation

## Resource Creation Failures

### Symptoms

- "already exists" errors
- Namespace conflicts
- PipelineRun not created

### Log Pattern

```
"error": "resource already exists"
"msg": "failed to create PipelineRun"
```

### Investigation

```bash
grep -i "already exists\|conflict\|failed to create" pipelines-as-code-controller.log
grep -i "error\|warning" events
```

### Common Causes

- Test cleanup didn't complete
- Namespace collision
- CRD not installed

## Git Provider API Errors

### Symptoms

- Cannot fetch pipeline files
- Status update failures
- Comment creation fails

### Log Pattern

```
"msg": "failed to get file content"
"msg": "failed to create status"
"status": 404
```

### Investigation

```bash
grep -i "failed to\|could not\|404\|500" pipelines-as-code-controller.log | grep -v "level.*debug"
```

### Common Causes

- File not in expected location
- Branch/ref doesn't exist
- Repository permissions

## Kubernetes Events Errors

### Symptoms

- Pods not starting
- ImagePullBackOff
- OOMKilled

### Investigation

```bash
grep -i "error\|failed\|backoff\|oom" events
```

### Common Causes

- Image not available
- Resource quotas exceeded
- Node pressure

## Flaky Test Indicators

### Timing-Related

- Test passes on retry
- Failure message includes "timeout" or "deadline"
- Different failure point on each run

### Resource Contention

- Multiple tests using same namespace prefix
- Parallel test interference
- Shared cluster state

### Investigation for Flakiness

```bash
# Check if failure is consistent
grep "--- FAIL" e2e-test-output.log

# Look for retry patterns
grep -i "retry\|retrying" pipelines-as-code-controller.log
```
