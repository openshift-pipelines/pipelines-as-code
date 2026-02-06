---
name: e2e-artifacts
description: >-
  This skill should be used when the user asks about "e2e artifacts", "what's in e2e logs",
  "e2e log structure", "e2e files", "artifact contents", or needs a quick reference for
  what each file in the E2E test artifacts contains and when to use it.
version: 0.1.0
---

# E2E Test Artifacts Reference

Quick reference for the files available in downloaded E2E test artifacts.

## Artifacts Location

Artifacts are typically downloaded to:

```
tmp/e2e/logs-e2e-tests-{pattern}-{timestamp}/
```

## Core Log Files

| File | Size | Purpose | When to Use |
|------|------|---------|-------------|
| `e2e-test-output.log` | ~500KB | Test execution output with PASS/FAIL results | **First file to check** - find which tests failed |
| `pipelines-as-code-controller.log` | 3-10MB | Main controller debug logs | Investigate processing errors, API calls, reconciliation |
| `pipelines-as-code-watcher.log` | ~500KB | Watcher component logs | Check PipelineRun status tracking |
| `pipelines-as-code-webhook.log` | ~200KB | Webhook component logs | Investigate webhook reception/validation |
| `pac-pods.log` | ~1MB | Combined PAC pod logs | Quick overview of all components |

## Kubernetes Resources

| File | Purpose | When to Use |
|------|---------|-------------|
| `pac-pipelineruns.yaml` | All PipelineRun resources | Check pipeline execution status, conditions |
| `pac-repositories.yaml` | Repository CRDs | Verify repository configuration |
| `events` | Kubernetes events | Find scheduling issues, pod failures |

## Special Directories

### `api-instrumentation/`

Contains per-test JSON files with API call metrics:

- Request counts
- Response times
- Rate limit status
- Error responses

**Use when**: Investigating rate limiting or API failures

### `gosmee/`

Webhook relay logs:

- `main.log` - Relay status and errors
- Payload files for debugging webhook content

**Use when**: Webhooks not being received

### `ns/`

Per-namespace resource dumps:

- ConfigMaps, Secrets (redacted)
- Pods, Services
- PipelineRuns, TaskRuns

**Use when**: Need detailed resource state for specific test namespace

## Quick Commands

### Find failed tests

```bash
grep -B10 -- "--- FAIL.*Test" e2e-test-output.log
```

### Find errors in controller

```bash
grep '"level":"error"' pipelines-as-code-controller.log
```

### Search for namespace across all logs

```bash
grep -r "pac-e2e-ns-xxxxx" .
```

### Check for rate limits

```bash
grep -i "rate.limit\|403\|429" pipelines-as-code-controller.log
```

## See Also

For detailed investigation workflow, use the `/e2e-investigate` skill.
