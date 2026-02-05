# E2E Artifacts Directory Structure

Complete reference for the E2E test artifacts directory layout.

## Directory Tree

```
logs-e2e-tests-{pattern}-{timestamp}/
├── e2e-test-output.log           # Test execution output
├── pipelines-as-code-controller.log  # Controller logs
├── pipelines-as-code-watcher.log     # Watcher logs
├── pipelines-as-code-webhook.log     # Webhook logs
├── pac-pods.log                  # Combined pod logs
├── pac-pipelineruns.yaml         # PipelineRun resources
├── pac-repositories.yaml         # Repository CRDs
├── events                        # Kubernetes events
├── api-instrumentation/          # Per-test API metrics
│   ├── TestGithubPullRequest.json
│   ├── TestGiteaPush.json
│   └── ...
├── gosmee/                       # Webhook relay logs
│   ├── main.log
│   └── payloads/
│       └── ...
└── ns/                           # Per-namespace dumps
    ├── pac-e2e-ns-xxxxx/
    │   ├── pods.yaml
    │   ├── pipelineruns.yaml
    │   ├── taskruns.yaml
    │   ├── configmaps.yaml
    │   └── ...
    └── ...
```

## File Details

### e2e-test-output.log

**Format**: Mixed text and JSON
**Size**: ~500KB
**Content**:

- Test start/stop markers
- Assertion results
- Timeout messages
- PASS/FAIL status for each test

**Example content**:

```
=== RUN   TestGithubPullRequest
    e2e_test.go:123: Setting up test in namespace pac-e2e-ns-abc123
    e2e_test.go:145: Waiting for PipelineRun to complete...
--- PASS: TestGithubPullRequest (45.23s)
=== RUN   TestGiteaPush
    e2e_test.go:200: Error: timeout waiting for status
--- FAIL: TestGiteaPush (120.00s)
```

### pipelines-as-code-controller.log

**Format**: JSON (one object per line)
**Size**: 3-10MB
**Content**:

- Webhook processing
- PipelineRun creation
- Git provider API calls
- Status updates
- Error messages

**Example log entry**:

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00.000Z",
  "logger": "pipelinesascode",
  "msg": "processing incoming webhook",
  "namespace": "pac-e2e-ns-abc123",
  "event-type": "pull_request",
  "event-id": "12345"
}
```

### pipelines-as-code-watcher.log

**Format**: JSON
**Size**: ~500KB
**Content**:

- PipelineRun status changes
- Completion events
- Cleanup operations

### pipelines-as-code-webhook.log

**Format**: JSON
**Size**: ~200KB
**Content**:

- Incoming webhook requests
- Payload validation
- Signature verification
- Routing decisions

### pac-pipelineruns.yaml

**Format**: YAML (multiple documents)
**Content**: All PipelineRun resources from test namespaces

**Key fields to check**:

```yaml
status:
  conditions:
    - type: Succeeded
      status: "False"
      reason: Failed
      message: "TaskRun xyz failed"
  startTime: "2024-01-15T10:30:00Z"
  completionTime: "2024-01-15T10:32:00Z"
```

### pac-repositories.yaml

**Format**: YAML (multiple documents)
**Content**: Repository CRD resources

**Key fields**:

```yaml
spec:
  url: "https://github.com/owner/repo"
  git_provider:
    type: github
    secret:
      name: pac-secret
```

### events

**Format**: Plain text (kubectl get events output)
**Content**: Kubernetes events from test namespaces

**Example**:

```
NAMESPACE          LAST SEEN   TYPE      REASON    OBJECT                     MESSAGE
pac-e2e-ns-abc123  2m          Warning   Failed    pod/pipelinerun-xyz-pod    Error: ImagePullBackOff
```

### api-instrumentation/*.json

**Format**: JSON
**Content**: Per-test API call statistics

**Example**:

```json
{
  "test_name": "TestGithubPullRequest",
  "total_requests": 45,
  "rate_limit_remaining": 4500,
  "errors": [],
  "duration_ms": 45230
}
```

### gosmee/main.log

**Format**: Text with timestamps
**Content**: Webhook relay status

**Example**:

```
2024-01-15 10:30:00 INFO  Received webhook for repo owner/repo
2024-01-15 10:30:00 INFO  Forwarded to http://webhook-service:8080
2024-01-15 10:30:01 ERROR Connection refused to target
```

### ns/{namespace}/*.yaml

**Format**: YAML
**Content**: Kubernetes resources for specific test namespace

Includes:

- `pods.yaml` - Pod definitions and status
- `pipelineruns.yaml` - Namespace-specific PipelineRuns
- `taskruns.yaml` - TaskRun resources
- `configmaps.yaml` - ConfigMaps (may be redacted)
- `secrets.yaml` - Secret metadata (values redacted)
