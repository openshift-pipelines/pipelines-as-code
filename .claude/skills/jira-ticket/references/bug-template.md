# SRVKP Jira Bug Report Template

Complete template for creating SRVKP Jira bug reports. Copy this template and fill in the placeholders with actual content.

## Template

```jira
h3. *Description of problem:*

<What is broken or incorrect>

Workaround: <If available, describe temporary solution>

h3. *Prerequisites (if any, like setup, operators/versions):*

<Required environment, versions, setup>

h3. *Steps to Reproduce*

# <step 1>
# <step 2>
# <step 3>

h3. *Actual results:*

<What actually happens>

h3. *Expected results:*

<What should happen>

h3. *Reproducibility (Always/Intermittent/Only Once):*

<Frequency of reproduction>

h3. *Acceptance criteria:*

<What must be true when bug is fixed>

*Definition of Done:*

<Specific done criteria for this bug>

h3. *Build Details:*

<Version, commit, build number, environment>

h3. *Additional info (Such as Logs, Screenshots, etc):*

<Logs, stack traces, screenshots, debugging info>
```

## Section-by-Section Guide

### Description of Problem

**Purpose**: Clearly describe what is broken or incorrect

**Content**:

- What is broken or not working
- Impact on users or system
- Severity (crash, data loss, UX issue, cosmetic)
- When the problem was first observed
- Optional: Workaround if available

**Examples**:

```jira
h3. *Description of problem:*

Webhook controller crashes with nil pointer dereference when receiving invalid JSON payload from GitHub webhooks. This causes all webhook processing to halt and requires manual controller restart. Affects all webhook events during the crash window.

Workaround: Ensure all webhook payloads are valid JSON before sending. No workaround for malicious or malformed external requests.
```

```jira
h3. *Description of problem:*

Pipeline runs fail to start when repository name contains special characters (e.g., "my-repo@v2", "test&dev"). The repository is correctly synced but PipelineRun creation fails with validation error. This blocks approximately 15% of repositories in the organization.

Workaround: Rename repository to remove special characters (not always feasible for users).
```

```jira
h3. *Description of problem:*

Log messages from webhook handler are missing timestamps, making debugging extremely difficult. All other components include timestamps, but webhook logs show only message text without time information. This significantly increases Mean Time To Resolution (MTTR) for webhook-related issues.

Workaround: None. Must correlate with proxy logs to determine timing.
```

### Prerequisites

**Purpose**: Define required environment and setup

**Content**:

- Version information (operators, CRDs, dependencies)
- Required setup or configuration
- Environment details (cluster version, OS, architecture)
- Specific conditions needed to reproduce
- External dependencies or services

**Examples**:

```jira
h3. *Prerequisites (if any, like setup, operators/versions):*

* Pipelines-as-Code v0.21.0
* Tekton Pipelines v0.50.0
* Kubernetes 1.27+
* GitHub webhook configured and active
* Repository with webhook secret enabled
```

```jira
h3. *Prerequisites (if any, like setup, operators/versions):*

* OpenShift Pipelines 1.12.0 (includes Pipelines-as-Code v0.19.0)
* OpenShift 4.13
* Repository CRD with special characters in name: {{my-repo@v2}}
* At least one .tekton/*.yaml file in repository
* GitHub App authentication configured
```

```jira
h3. *Prerequisites (if any, like setup, operators/versions):*

* Any version of Pipelines-as-Code (bug exists in all versions)
* Webhook handler deployment running
* Any Git provider (GitHub, GitLab, Gitea)
* Log aggregation tool (for observing missing timestamps)
```

### Steps to Reproduce

**Purpose**: Provide exact steps to reproduce the issue

**Content**:

- Numbered, sequential steps
- Specific commands, API calls, or UI actions
- Include exact inputs and parameters
- Be detailed enough for someone else to reproduce
- Note any timing or ordering requirements

**Examples**:

```jira
h3. *Steps to Reproduce*

# Deploy Pipelines-as-Code v0.21.0
# Configure GitHub webhook for any repository
# Send malformed JSON payload to webhook endpoint:
{noformat}
curl -X POST https://pac-webhook/github \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=abc123" \
  -d '{"incomplete": "json", no closing brace'
{noformat}
# Observe controller logs for crash
# Attempt to send valid webhook - fails because controller is crashed
```

```jira
h3. *Steps to Reproduce*

# Create GitHub repository with name containing @ symbol: {{my-repo@v2}}
# Create Repository CRD:
{code:yaml}
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: my-repo-at-v2
spec:
  url: "https://github.com/org/my-repo@v2"
{code}
# Add .tekton/pipeline.yaml to repository
# Trigger webhook by pushing commit
# Observe PipelineRun fails to create
# Check controller logs for validation error
```

```jira
h3. *Steps to Reproduce*

# Deploy Pipelines-as-Code
# Configure webhook for any repository
# Trigger webhook event (push, PR, etc.)
# View webhook handler logs:
{noformat}
kubectl logs deployment/pipelines-as-code-webhook
{noformat}
# Observe log entries missing timestamp field
# Compare with controller logs which do include timestamps
```

### Actual Results

**Purpose**: Describe what actually happens (incorrect behavior)

**Content**:

- Exact error messages (in code blocks)
- Observed behavior
- Screenshots if UI-related
- Stack traces if crashes
- Log excerpts showing the problem
- Metrics or data showing incorrect state

**Examples**:

```jira
h3. *Actual results:*

Controller crashes immediately upon receiving invalid JSON:

{noformat}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]

goroutine 42 [running]:
pkg/webhook.parsePayload(0xc000123456)
    webhook/handler.go:123 +0x45
pkg/webhook.HandleGitHub(0xc000654321)
    webhook/github.go:78 +0x123
net/http.HandlerFunc.ServeHTTP(0xc000999888)
    http/server.go:2109 +0x45
{noformat}

All webhook processing stops. Controller pod status changes to CrashLoopBackoff. Subsequent webhooks fail with 503 Service Unavailable. Manual pod restart required to restore functionality.
```

```jira
h3. *Actual results:*

PipelineRun creation fails with validation error:

{code}
ERROR: Repository name "my-repo@v2" is invalid:
spec.url: Invalid value: "https://github.com/org/my-repo@v2":
must match regex: ^[a-zA-Z0-9._-]+$
{code}

No PipelineRun is created. Repository remains synced but non-functional. Users receive webhook delivery failure notification from GitHub. Pipeline never runs despite valid configuration.
```

```jira
h3. *Actual results:*

Webhook handler logs show messages without timestamps:

{noformat}
level=info msg="Processing GitHub webhook"
level=debug msg="Validating signature"
level=info msg="Signature valid"
level=info msg="Creating PipelineRun"
level=info msg="PipelineRun created successfully"
{noformat}

Cannot determine when events occurred. Must cross-reference with API server logs or GitHub webhook delivery logs to understand timing. Makes debugging time-sensitive issues nearly impossible.
```

### Expected Results

**Purpose**: Describe what should happen (correct behavior)

**Content**:

- Expected correct behavior
- How system should respond
- What success looks like
- Expected log messages or output
- Expected state changes

**Examples**:

```jira
h3. *Expected results:*

Controller should handle invalid JSON gracefully:

* Parse error is caught and logged with helpful message
* HTTP 400 Bad Request response returned to webhook sender
* Response body includes error details: "Invalid JSON payload: unexpected end of input"
* Controller continues processing other webhooks normally
* No crash, no pod restart required
* Metrics counter incremented for invalid payloads

Example log entry:
{code:json}
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "warn",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Failed to parse webhook payload",
  "error": "invalid JSON: unexpected end of input",
  "remote_addr": "192.168.1.100"
}
{code}
```

```jira
h3. *Expected results:*

Repository with special characters in name should work correctly:

* Repository CRD accepted by API server
* URL validated correctly (special chars allowed in URL path)
* PipelineRun created successfully when webhook received
* Pipeline runs normally with correct parameters
* No validation errors

Or, if special characters are truly unsupported:
* Clear error message during Repository creation explaining limitation
* Documentation clearly states naming requirements
* Suggested alternative naming convention
```

```jira
h3. *Expected results:*

Webhook handler logs should include timestamps like all other components:

{noformat}
timestamp=2024-01-15T10:30:00.123Z level=info msg="Processing GitHub webhook" correlation_id=550e8400
timestamp=2024-01-15T10:30:00.145Z level=debug msg="Validating signature"
timestamp=2024-01-15T10:30:00.167Z level=info msg="Signature valid"
timestamp=2024-01-15T10:30:00.234Z level=info msg="Creating PipelineRun"
timestamp=2024-01-15T10:30:00.456Z level=info msg="PipelineRun created successfully"
{noformat}

Timestamps should:
* Use RFC3339 format with milliseconds
* Be in UTC timezone
* Appear as first field in each log line
* Match timestamp format used by other components
```

### Reproducibility

**Purpose**: Indicate how often the bug occurs

**Content**:

- **Always**: Happens every time under described conditions
- **Intermittent**: Happens sometimes but not always
- **Only Once**: Happened once, cannot reproduce

Include additional details:

- For "Always": Confirm tested multiple times
- For "Intermittent": Approximate frequency (50%, 1/10 times, etc.)
- For "Only Once": Why unable to reproduce, what might have changed

**Examples**:

```jira
h3. *Reproducibility (Always/Intermittent/Only Once):*

Always - Tested 20 times, crash occurs every time invalid JSON is sent to webhook endpoint.
```

```jira
h3. *Reproducibility (Always/Intermittent/Only Once):*

Intermittent - Occurs approximately 30% of the time. More likely to occur under load (>50 concurrent webhooks). May be related to race condition in validation code.
```

```jira
h3. *Reproducibility (Always/Intermittent/Only Once):*

Always - Tested with 10 different repository names containing various special characters (@, &, $, %, #). All fail with validation error 100% of the time.
```

```jira
h3. *Reproducibility (Always/Intermittent/Only Once):*

Only Once - Occurred during production incident on 2024-01-15. Unable to reproduce in staging or development environments. Potentially related to specific webhook payload structure that was not preserved. Recommended increasing webhook payload logging to help future debugging.
```

### Acceptance Criteria

**Purpose**: Define what must be true when bug is fixed

**Content**:

- Fix verification steps
- Regression test requirements
- Performance criteria if relevant
- Security requirements if relevant
- Specific scenarios to test

**Examples**:

```jira
h3. *Acceptance criteria:*

* Invalid JSON payloads do not crash controller
* HTTP 400 response returned with clear error message
* Error logged with correlation ID and payload details (sanitized)
* Controller remains operational and processes subsequent webhooks
* Unit test added covering invalid JSON handling
* Integration test verifies end-to-end behavior
* Manual test: send 100 invalid payloads, verify 0 crashes
* Metrics counter added for invalid_payload_errors
* No performance degradation for valid payloads

*Definition of Done:*

* Bug fix merged and deployed to staging environment
* All acceptance tests passing
* No controller crashes observed in 7-day staging soak test
* Unit and integration tests provide regression coverage
* Runbook updated with troubleshooting steps for invalid payloads
```

```jira
h3. *Acceptance criteria:*

* Repositories with special characters in name are either:
  ** Supported: Create and function correctly, OR
  ** Rejected: Clear error message at creation time
* If supported: PipelineRuns created successfully for all special chars
* If rejected: API validation prevents creation with helpful message
* Documentation updated with naming requirements
* Unit tests cover edge cases (all special characters)
* E2E test verifies real repository with special character
* No breaking changes to existing repositories

*Definition of Done:*

* Decision made: support or reject special characters
* Implementation complete and tested
* Documentation reflects naming requirements
* Regression tests prevent future breakage
* Existing users not impacted (backwards compatible)
```

```jira
h3. *Acceptance criteria:*

* All webhook handler log entries include timestamp field
* Timestamp format matches other components (RFC3339 with milliseconds)
* Timestamp is first field in log line
* UTC timezone used consistently
* No performance impact from timestamp addition
* Structured logging format maintained
* Log aggregation correctly parses new format

*Definition of Done:*

* Webhook handler logging updated
* All log entries include proper timestamps
* Verified in log aggregation UI (Kibana/Grafana)
* No log parsing errors in aggregation system
* Documentation updated if log format changed
```

### Build Details

**Purpose**: Provide version and environment information

**Content**:

- Software versions (exact versions, not "latest")
- Commit hashes if known
- Build numbers
- Container image tags
- Environment details (cloud provider, region, cluster version)
- Architecture (amd64, arm64)

**Examples**:

```jira
h3. *Build Details:*

* Pipelines-as-Code: v0.21.0
* Commit: abc123def456789
* Container image: quay.io/openshift-pipelines/pipelines-as-code-controller:v0.21.0
* Tekton Pipelines: v0.50.0
* Kubernetes: v1.27.3
* Platform: OpenShift 4.13.0
* Architecture: amd64
* Region: AWS us-east-1
```

```jira
h3. *Build Details:*

* OpenShift Pipelines Operator: 1.12.0
* Pipelines-as-Code: v0.19.0 (bundled with operator)
* OpenShift: 4.13.0
* Cluster: Production cluster (https://console.prod.example.com)
* Observed: 2024-01-15 10:30 UTC
* Environment: Red Hat OpenShift on Azure
```

```jira
h3. *Build Details:*

* All versions (bug is version-independent)
* Tested on: v0.19.0, v0.20.0, v0.21.0
* Platform: Kubernetes 1.26, 1.27, 1.28
* Architecture: Both amd64 and arm64
* Issue exists since initial webhook handler implementation
```

### Additional Info

**Purpose**: Provide debugging details

**Content**:

- Complete stack traces
- Full log excerpts (use code blocks)
- Screenshots (for UI issues)
- Metrics or monitoring data
- Related issues or bugs
- Debugging steps already attempted
- Network traces if relevant

**Examples**:

```jira
h3. *Additional info (Such as Logs, Screenshots, etc):*

*Full stack trace:*
{code}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]

goroutine 42 [running]:
pkg/webhook.parsePayload(0xc000123456)
    /workspace/pkg/webhook/handler.go:123 +0x45
    payload := gjson.Parse(body) // body is nil here

pkg/webhook.HandleGitHub(0xc000654321, 0xc000111222)
    /workspace/pkg/webhook/github.go:78 +0x123
    err := parsePayload(req.Body)

net/http.HandlerFunc.ServeHTTP(0xc000999888, 0xc000777666, 0xc000555444)
    /usr/local/go/src/net/http/server.go:2109 +0x45
{code}

*Controller logs before crash:*
{noformat}
2024-01-15T10:29:58Z INFO Starting webhook handler
2024-01-15T10:30:00Z INFO Received webhook from 192.168.1.100
2024-01-15T10:30:00Z DEBUG Content-Type: application/json
2024-01-15T10:30:00Z DEBUG X-Hub-Signature-256: sha256=abc123...
2024-01-15T10:30:00Z FATAL panic in webhook handler [stack trace above]
{noformat}

*Malformed payload that triggers crash:*
{code:json}
{"incomplete": "json", no closing brace
{code}

*Related issues:*
* Similar crash reported in SRVKP-XXX for GitLab webhooks
* Input validation added for other endpoints in SRVKP-YYY

*Attempted debugging:*
* Confirmed crash with minimal reproduction case
* Tested with various malformed JSON patterns (all crash)
* Added defensive nil checks locally - prevents crash
* Root cause: parsePayload assumes non-nil body without validation
```

## Complete Example: Bug Report for Webhook Controller Crash

```jira
h3. *Description of problem:*

Webhook controller crashes with nil pointer dereference when receiving invalid JSON payload from GitHub webhooks. This causes complete webhook processing failure and requires manual controller pod restart. All webhook events are lost during the crash window, potentially missing critical pipeline triggers.

Impact: HIGH - Affects all webhook processing, requires manual intervention

Workaround: Ensure all webhook payloads are valid JSON before sending. However, this provides no protection against malicious or malformed external requests from GitHub.

h3. *Prerequisites (if any, like setup, operators/versions):*

* Pipelines-as-Code v0.21.0
* Tekton Pipelines v0.50.0
* Kubernetes 1.27+ or OpenShift 4.13+
* GitHub webhook configured and actively sending events
* Repository with webhook secret configured
* GitHub App or webhook token authentication enabled

h3. *Steps to Reproduce*

# Deploy Pipelines-as-Code v0.21.0 to Kubernetes cluster
# Configure GitHub webhook for any repository
# Send malformed JSON payload to webhook endpoint:
{noformat}
curl -X POST https://pac-webhook.example.com/github \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=abc123def456" \
  -d '{"incomplete": "json", no closing brace'
{noformat}
# Observe controller logs for panic and crash
# Verify controller pod enters CrashLoopBackoff state
# Attempt to send valid webhook - fails with 503 Service Unavailable
# Manually restart controller pod to restore functionality

h3. *Actual results:*

Controller crashes immediately upon receiving invalid JSON:

{code}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]

goroutine 42 [running]:
pkg/webhook.parsePayload(0xc000123456)
    /workspace/pkg/webhook/handler.go:123 +0x45
pkg/webhook.HandleGitHub(0xc000654321, 0xc000111222)
    /workspace/pkg/webhook/github.go:78 +0x123
net/http.HandlerFunc.ServeHTTP(...)
    /usr/local/go/src/net/http/server.go:2109 +0x45
{code}

Consequences:
* Controller pod crashes and enters CrashLoopBackoff
* All webhook processing stops (not just the malformed request)
* Subsequent valid webhooks fail with 503 error
* Manual pod restart required (kubectl rollout restart)
* Webhook events lost during crash window
* GitHub marks webhooks as failed, may trigger retries

h3. *Expected results:*

Controller should handle invalid JSON gracefully without crashing:

*Error Handling:*
* Parse error caught before accessing payload fields
* HTTP 400 Bad Request returned to GitHub
* Response body explains error: {{"error": "Invalid JSON payload", "details": "unexpected end of input"}}
* Error logged with details (sanitized to prevent log injection)

*Operational Continuity:*
* Controller continues running normally
* Subsequent valid webhooks processed successfully
* No manual intervention required
* No pod restarts

*Observability:*
* Error logged with correlation ID, timestamp, remote IP
* Metrics counter incremented: {{webhook_invalid_payload_total}}
* Alert triggered if error rate exceeds threshold

*Example log entry:*
{code:json}
{
  "timestamp": "2024-01-15T10:30:00.123Z",
  "level": "warn",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Failed to parse webhook payload",
  "error": "invalid JSON: unexpected end of input at position 23",
  "remote_addr": "192.168.1.100",
  "user_agent": "GitHub-Hookshot/abc123"
}
{code}

h3. *Reproducibility (Always/Intermittent/Only Once):*

Always - Tested 20 times with various malformed JSON payloads. Crash occurs 100% of the time. Tested with:
* Incomplete JSON (missing closing braces)
* Invalid characters in JSON
* Non-JSON content with JSON content-type header
* Empty payload
* Extremely large payloads (>10MB)

All cases result in controller crash.

h3. *Acceptance criteria:*

*Fix Verification:*
* Invalid JSON payloads do not crash controller (tested with 10 malformed payload types)
* HTTP 400 response returned with clear, actionable error message
* Error logged with correlation ID, timestamp, sanitized payload sample
* Controller remains operational and processes subsequent valid webhooks
* No pod restarts occur due to invalid payloads

*Testing Requirements:*
* Unit test added: TestInvalidJSONHandling
* Integration test added: TestWebhookRobustness
* Manual test: send 100 consecutive invalid payloads, verify 0 crashes
* Load test: send mix of valid (80%) and invalid (20%) under load, verify stability

*Monitoring and Metrics:*
* Prometheus metric added: {{webhook_invalid_payload_total}}
* Alert configured if error rate >10% over 5 minutes
* Grafana dashboard shows invalid payload trends

*Security:*
* No sensitive data logged (payloads sanitized)
* Error messages don't leak internal details
* Rate limiting prevents DoS via invalid payloads

*Performance:*
* No performance degradation for valid payload processing
* Invalid payload handling completes in <10ms
* Memory usage stable under invalid payload load

*Definition of Done:*

* Bug fix merged to main branch
* All acceptance tests passing in CI
* Deployed to staging environment
* 7-day soak test shows 0 crashes under mixed load
* Documentation updated (troubleshooting guide)
* Runbook includes section on invalid payload errors
* Security review completed (for error message content)

h3. *Build Details:*

* Pipelines-as-Code: v0.21.0
* Git commit: abc123def456789012345678901234567890abcd
* Container image: quay.io/openshift-pipelines/pipelines-as-code-controller:v0.21.0
* Image digest: sha256:fedcba9876543210...
* Tekton Pipelines: v0.50.0
* Tekton Triggers: v0.24.0
* Kubernetes: v1.27.3
* Platform: OpenShift 4.13.0
* Environment: Production cluster (us-east-1)
* Architecture: amd64
* Go version: 1.21.0
* First observed: 2024-01-15 10:30 UTC
* Occurrence frequency: 3 times in past 24 hours

h3. *Additional info (Such as Logs, Screenshots, etc):*

*Full stack trace with line numbers:*
{code}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]

goroutine 42 [running]:
pkg/webhook.parsePayload(0xc000123456)
    /workspace/pkg/webhook/handler.go:123 +0x45
    payload := gjson.Parse(body) // body is nil, should check first

pkg/webhook.HandleGitHub(0xc000654321, 0xc000111222)
    /workspace/pkg/webhook/github.go:78 +0x123

net/http.HandlerFunc.ServeHTTP(0xc000999888, 0xc000777666, 0xc000555444)
    /usr/local/go/src/net/http/server.go:2109 +0x45

net/http.serverHandler.ServeHTTP(...)
{code}

*Controller logs showing crash sequence:*
{noformat}
2024-01-15T10:29:58.123Z INFO  Starting webhook server port=8080
2024-01-15T10:29:58.456Z INFO  Webhook server ready
2024-01-15T10:30:00.123Z INFO  Received webhook remote_addr=192.168.1.100
2024-01-15T10:30:00.145Z DEBUG Content-Type=application/json
2024-01-15T10:30:00.167Z DEBUG X-Hub-Signature-256=sha256:abc123...
2024-01-15T10:30:00.189Z DEBUG Validating signature
2024-01-15T10:30:00.234Z DEBUG Signature valid
2024-01-15T10:30:00.256Z DEBUG Parsing payload
2024-01-15T10:30:00.267Z FATAL panic in webhook handler error="runtime error: invalid memory address or nil pointer dereference"
[stack trace as above]
{noformat}

*Malformed JSON payloads that trigger crash:*
{code}
# Missing closing brace
{"incomplete": "json", no closing

# Invalid character
{"invalid": "char"} extra content here

# Empty payload
[empty]

# Non-JSON with JSON content-type
This is not JSON at all

# Truncated JSON
{"foo": "bar", "nested": {"incomplete
{code}

*Problematic code in handler.go:123:*
{code:go}
func parsePayload(r *http.Request) (*GitHubPayload, error) {
    body, _ := ioutil.ReadAll(r.Body) // Error ignored
    payload := gjson.Parse(string(body)) // Crashes if body is nil or invalid
    // No validation here
    return &GitHubPayload{
        Action: payload.Get("action").String(), // NPE if payload parse failed
    }, nil
}
{code}

*Suggested fix (tested locally):*
{code:go}
func parsePayload(r *http.Request) (*GitHubPayload, error) {
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read request body: %w", err)
    }

    if len(body) == 0 {
        return nil, errors.New("empty request body")
    }

    if !json.Valid(body) {
        return nil, fmt.Errorf("invalid JSON payload: %s",
            string(body[:min(len(body), 100)])) // Limit for logging
    }

    payload := gjson.ParseBytes(body)
    if !payload.IsObject() {
        return nil, errors.New("payload is not a JSON object")
    }

    return &GitHubPayload{
        Action: payload.Get("action").String(),
    }, nil
}
{code}

*Related issues:*
* SRVKP-XXX: Similar crash in GitLab webhook handler (fixed in v0.20.0)
* SRVKP-YYY: Added input validation framework for API endpoints
* Upstream issue: https://github.com/example/repo/issues/123

*Attempted workarounds:*
* Added webhook validation in GitHub settings (doesn't prevent malicious requests)
* Implemented external proxy for JSON validation (adds latency and complexity)
* Increased pod memory (doesn't fix crash, just delays it)

*Impact metrics:*
* 3 production incidents in past 24 hours
* Average downtime per incident: 5 minutes
* Estimated webhooks lost: ~50 events per incident
* Manual intervention required: 3 pod restarts
* User impact: Pipeline triggers delayed or missed
```

## Tips for Writing Good Bug Reports

1. **Be specific and detailed**: More information helps faster resolution
2. **Include reproduction steps**: Exact steps, not general descriptions
3. **Show actual vs expected**: Clear contrast highlights the bug
4. **Provide context**: Version numbers, environment details matter
5. **Include logs and errors**: Complete stack traces and error messages
6. **Test reproducibility**: Try multiple times, note frequency
7. **Think about acceptance criteria**: How do we know it's really fixed?
8. **Use Jira markdown**: Remember h3., {code}, {noformat}, etc.

## Common Mistakes

❌ **Vague description**: "It's broken"
✓ **Specific description**: "Controller crashes with nil pointer dereference when receiving invalid JSON"

❌ **Missing reproduction**: "Send a webhook"
✓ **Detailed reproduction**: Exact curl command with headers and payload

❌ **No logs**: "It crashed"
✓ **Complete logs**: Full stack trace with line numbers and context

❌ **Standard markdown in code**: ` ```json...``` `
✓ **Jira code blocks**: `{code:json}...{code}`

❌ **"Works on my machine"**: No environment details
✓ **Complete build details**: Versions, architecture, platform, image tags
