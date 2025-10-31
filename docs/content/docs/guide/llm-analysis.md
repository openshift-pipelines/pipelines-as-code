---
title: AI/LLM-Powered Pipeline Analysis
weight: 42
---

# AI/LLM-Powered Pipeline Analysis

{{< tech_preview >}}

Pipelines as Code supports AI-powered analysis of your CI/CD pipeline runs using Large Language Models (LLMs). This feature can automatically analyze failures, provide insights, and suggest fixes directly in your pull requests.

## Overview

The LLM analysis feature enables you to:

- **Automatically analyze failed pipelines** and provide root cause analysis
- **Generate actionable recommendations** for fixing issues
- **Post insights as PR comments**
- **Configure custom analysis scenarios** using different prompts and triggers

> **Note**: Additional output destinations (`check-run` and `annotation`) and structured JSON output are planned for future releases.

## Supported Providers

- **OpenAI** - Default model: `gpt-5-mini`
- **Google Gemini** - Default model: `gemini-2.5-flash-lite`

You can specify any model supported by your chosen provider. See [Model Selection](#model-selection) for guidance.

## Configuration

LLM analysis is configured in the `Repository` CRD under `spec.settings.ai`:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: my-repo
spec:
  url: "https://github.com/org/repo"
  settings:
    ai:
      enabled: true
      provider: "openai"
      timeout_seconds: 30
      max_tokens: 1000
      secret_ref:
        name: "openai-api-key"
        key: "token"
      roles:
        - name: "failure-analysis"
          model: "gpt-5-mini"  # Optional: specify model (uses provider default if omitted)
          prompt: |
            You are a DevOps expert. Analyze this failed pipeline and:
            1. Identify the root cause
            2. Suggest specific fixes
            3. Recommend preventive measures
          on_cel: 'body.pipelineRun.status.conditions[0].reason == "Failed"'
          context_items:
            error_content: true
            container_logs:
              enabled: true
              max_lines: 100
          output: "pr-comment"
```

## Configuration Fields

### Top-Level Settings

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Enable/disable LLM analysis |
| `provider` | string | Yes | LLM provider: `openai` or `gemini` |
| `api_url` | string | No | Custom API endpoint URL (overrides provider default) |
| `timeout_seconds` | integer | No | Request timeout (1-300, default: 30) |
| `max_tokens` | integer | No | Maximum response tokens (1-4000, default: 1000) |
| `secret_ref` | object | Yes | Reference to Kubernetes secret with API key |
| `roles` | array | Yes | List of analysis scenarios (minimum 1) |

### Analysis Roles

Each role defines a specific analysis scenario:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier for this role |
| `prompt` | string | Yes | Prompt template for the LLM |
| `model` | string | No | Model name (consult provider documentation for available models). Uses provider default if not specified. |
| `on_cel` | string | No | CEL expression for conditional triggering. If not specified, the role will always run. |
| `output` | string | Yes | Output destination (currently only `pr-comment` is supported) |
| `context_items` | object | No | Configuration for context inclusion |

### Context Items

Control what information is sent to the LLM:

| Field | Type | Description |
|-------|------|-------------|
| `commit_content` | boolean | Include commit information (see Commit Fields below) |
| `pr_content` | boolean | Include PR title, description, metadata |
| `error_content` | boolean | Include error messages and failures |
| `container_logs.enabled` | boolean | Include container/task logs |
| `container_logs.max_lines` | integer | Limit log lines (1-1000, default: 50). ⚠️ High values may impact performance |

#### Commit Fields

When `commit_content: true` is enabled, the following fields are included in the LLM context:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `commit.sha` | string | Commit SHA hash | `"abc123def456..."` |
| `commit.message` | string | Commit title (first line/paragraph) | `"feat: add new feature"` |
| `commit.url` | string | Web URL to view the commit | `"https://github.com/org/repo/commit/abc123"` |
| `commit.full_message` | string | Complete commit message (if different from title) | `"feat: add new feature\n\nDetailed description..."` |
| `commit.author.name` | string | Author's name | `"John Doe"` |
| `commit.author.date` | timestamp | When the commit was authored | `"2024-01-15T10:30:00Z"` |
| `commit.committer.name` | string | Committer's name (may differ from author) | `"GitHub"` |
| `commit.committer.date` | timestamp | When the commit was committed | `"2024-01-15T10:31:00Z"` |

**Privacy & Security Notes:**

- **Email addresses are intentionally excluded** from the commit context to protect personally identifiable information (PII) when sending data to external LLM APIs
- Fields are only included if available from the git provider
- Some providers may have limited information (e.g., Bitbucket Cloud only provides author name)
- Author and committer may be the same person or different (e.g., when using `git commit --amend` or rebasing)

## Model Selection

Each analysis role can specify a different model to optimize for your needs. If no model is specified, provider-specific defaults are used:

- **OpenAI**: `gpt-5-mini`
- **Gemini**: `gemini-2.5-flash-lite`

### Specifying Models

You can use any model name supported by your chosen provider. Consult your provider's documentation for available models:

- **OpenAI Models**: <https://platform.openai.com/docs/models>
- **Gemini Models**: <https://ai.google.dev/gemini-api/docs/models/gemini>

### Example: Per-Role Models

```yaml
settings:
  ai:
    enabled: true
    provider: "openai"
    secret_ref:
      name: "openai-api-key"
      key: "token"
    roles:
      # Use the most capable model for complex analysis
      - name: "security-analysis"
        model: "gpt-5"
        prompt: "Analyze security failures..."

      # Use default model (gpt-5-mini) for general analysis
      - name: "general-failure"
        # No model specified - uses provider default
        prompt: "Analyze this failure..."

      # Use the most economical model for quick checks
      - name: "quick-check"
        model: "gpt-5-nano"
        prompt: "Quick diagnosis..."
```

## CEL Expressions for Triggers

**By default, LLM analysis only runs for failed pipeline runs.** Use CEL expressions in `on_cel` to further control when analysis runs or to enable it for successful runs.

If `on_cel` is not specified, the role will execute for all failed pipeline runs.

### Overriding the Default Behavior

To run LLM analysis for **all pipeline runs** (both successful and failed), use `on_cel: 'true'`:

```yaml
roles:
  - name: "pipeline-summary"
    prompt: "Generate a summary of this pipeline run..."
    on_cel: 'true'  # Runs for ALL pipeline runs, not just failures
    output: "pr-comment"
```

This is useful for:

- Generating summaries for all pipeline runs
- Tracking metrics for successful runs
- Celebrating successes with automated messages
- Reporting on build performance

### Example CEL Expressions

```yaml
# Run on ALL pipeline runs (overrides default failed-only behavior)
on_cel: 'true'

# Only on successful runs (e.g., for generating success reports)
on_cel: 'body.pipelineRun.status.conditions[0].reason == "Succeeded"'

# Only on pull requests (in addition to default failed-only check)
on_cel: 'body.event.event_type == "pull_request"'

# Only on main branch
on_cel: 'body.event.base_branch == "main"'

# Only on default branch (works across repos with different default branches)
on_cel: 'body.event.base_branch == body.event.default_branch'

# Skip analysis for bot users
on_cel: 'body.event.sender != "dependabot[bot]"'

# Only for PRs with specific labels
on_cel: '"needs-review" in body.event.pull_request_labels'

# Only when triggered by comment
on_cel: 'body.event.trigger_comment.startsWith("/analyze")'

# Combine conditions
on_cel: 'body.pipelineRun.status.conditions[0].reason == "Failed" && body.event.event_type == "pull_request"'
```

### Available CEL Context Fields

#### Top-Level Context

| Field | Type | Description |
|-------|------|-------------|
| `body.pipelineRun` | object | Full PipelineRun object with status and metadata |
| `body.repository` | object | Full Repository CRD object |
| `body.event` | object | Event information (see Event Fields below) |
| `pac` | map[string]string | PAC parameters map |

#### Event Fields (`body.event.*`)

**Event Type and Trigger:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `event_type` | string | Event type from provider | `"pull_request"`, `"push"`, `"Merge Request Hook"` |
| `trigger_target` | string | Normalized trigger type across providers | `"pull_request"`, `"push"` |

**Branch and Commit Information:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `sha` | string | Commit SHA | `"abc123def456..."` |
| `sha_title` | string | Commit title/message | `"feat: add new feature"` |
| `base_branch` | string | Target branch for PR (or branch for push) | `"main"` |
| `head_branch` | string | Source branch for PR (or branch for push) | `"feature-branch"` |
| `default_branch` | string | Default branch of the repository | `"main"` or `"master"` |

**Repository Information:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `organization` | string | Organization/owner name | `"my-org"` |
| `repository` | string | Repository name | `"my-repo"` |

**URLs:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `url` | string | Web URL to repository | `"https://github.com/org/repo"` |
| `sha_url` | string | Web URL to commit | `"https://github.com/org/repo/commit/abc123"` |
| `base_url` | string | Web URL to base branch | `"https://github.com/org/repo/tree/main"` |
| `head_url` | string | Web URL to head branch | `"https://github.com/org/repo/tree/feature"` |

**User Information:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `sender` | string | User who triggered the event | `"user123"`, `"dependabot[bot]"` |

**Pull Request Fields (only populated for PR events):**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `pull_request_number` | int | PR/MR number | `42` |
| `pull_request_title` | string | PR/MR title | `"Add new feature"` |
| `pull_request_labels` | []string | List of PR/MR labels | `["enhancement", "needs-review"]` |

**Comment Trigger Fields (only when triggered by comment):**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `trigger_comment` | string | Comment that triggered the run | `"/test"`, `"/retest"` |

**Webhook Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `target_pipelinerun` | string | Target PipelineRun for incoming webhooks | `"my-pipeline-run"` |

### Excluded Fields

The following fields are **intentionally excluded** from CEL context for security and architectural reasons:

- **`event.Provider`** - Contains sensitive API tokens and webhook secrets
- **`event.Request`** - Contains raw HTTP headers and payload which may include secrets
- **`event.InstallationID`, `AccountID`, `GHEURL`, `CloneURL`** - Provider-specific internal identifiers and URLs
- **`event.SourceProjectID`, `TargetProjectID`** - GitLab-specific internal identifiers
- **`event.State`** - Internal state management fields
- **`event.Event`** - Raw provider event object (already represented in structured fields)

## Output Destinations

### PR Comment

Posts analysis as a comment on the pull request:

```yaml
output: "pr-comment"
```

Benefits:

- Visible to all developers
- Can be updated with new analysis
- Easy to discuss and follow up

> **Coming Soon**: Additional output destinations including `check-run` (GitHub check runs) and `annotation` (PipelineRun annotations) will be available in future releases.

## Setting Up API Keys

> **Important**: The Secret must be created in the same namespace as the Repository custom resource (CR).

### OpenAI

1. Get an API key from [OpenAI Platform](https://platform.openai.com/api-keys)

2. Create a Kubernetes secret:

```bash
kubectl create secret generic openai-api-key \
  --from-literal=token="sk-your-openai-api-key" \
  -n <namespace>
```

### Google Gemini

1. Get an API key from [Google AI Studio](https://makersuite.google.com/app/apikey)

2. Create a Kubernetes secret:

```bash
kubectl create secret generic gemini-api-key \
  --from-literal=token="your-gemini-api-key" \
  -n <namespace>
```

## Using Custom API Endpoints

The `api_url` field allows you to override the default API endpoint for LLM providers. This is useful for:

- Self-hosted LLM services (e.g., LocalAI, vLLM, Ollama with OpenAI adapter)
- Enterprise proxy services
- Regional or custom endpoints (e.g., Azure OpenAI)
- Alternative OpenAI-compatible APIs

### Example Configuration

```yaml
settings:
  ai:
    enabled: true
    provider: "openai"
    api_url: "https://custom-llm.example.com/v1"  # Custom endpoint
    secret_ref:
      name: "custom-api-key"
      key: "token"
    roles:
      - name: "failure-analysis"
        prompt: "Analyze this pipeline failure..."
        output: "pr-comment"
```

### Default API Endpoints

If `api_url` is not specified, these defaults are used:

- **OpenAI**: `https://api.openai.com/v1`
- **Gemini**: `https://generativelanguage.googleapis.com/v1beta`

### URL Format Requirements

The `api_url` must:

- Use `http://` or `https://` scheme
- Include a valid hostname
- Optionally include port and path components

Examples:

```yaml
# Valid URLs
api_url: "https://api.openai.com/v1"
api_url: "http://localhost:8080/v1"
api_url: "https://custom-proxy.company.com:9000/openai/v1"

# Invalid URLs
api_url: "ftp://example.com"       # Wrong scheme
api_url: "//example.com"           # Missing scheme
api_url: "not-a-url"               # Invalid format
```

## Example: Complete Configuration

See the [complete example](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/samples/repository-llm.yaml) for a full configuration with multiple roles.

## Best Practices

### Prompt Engineering

1. **Be specific**: Tell the LLM exactly what you want
2. **Structure your prompts**: Use numbered lists for clarity
3. **Set expectations**: Define the output format
4. **Provide context**: Explain what information will be provided

Example prompt:

```yaml
prompt: |
  You are a DevOps expert analyzing a CI/CD pipeline failure.

  Based on the error logs and context provided:
  1. Identify the root cause of the failure
  2. Suggest 2-3 specific steps to fix the issue
  3. Recommend one preventive measure for the future

  Keep your response concise and actionable.
```

### Security Considerations

1. **Protect API keys**: Always store in Kubernetes secrets
2. **Review logs**: Be aware of what logs are sent to external APIs
3. **Cost monitoring**: Set up billing alerts with your LLM provider
4. **Rate limiting**: Configure appropriate timeouts

### Cost Management

1. **Select appropriate models**: Use more economical models for simple tasks and reserve expensive models for complex analysis. Consult your provider's pricing documentation.
2. **Limit max_tokens**: Reduce costs by limiting response length
3. **Use selective triggers**: Only analyze failures, not all runs
4. **Control log lines**: Limit `max_lines` in container logs to reduce context size

### Performance Tips

1. **Set reasonable timeouts**: Default 30s is usually sufficient
2. **Non-blocking design**: Analysis runs in background, doesn't block pipeline
3. **Selective context**: Only include relevant context items
4. **Limit log fetching**: Setting `container_logs.max_lines` too high (>500) can impact performance when fetching logs from many containers. Start with lower values (50-100) and increase only if needed
5. **Monitor failures**: Check logs if analysis consistently fails

## Troubleshooting

### Analysis Not Running

Check that:

- `enabled: true` in configuration
- CEL expression in `on_cel` matches your event
- API key secret exists and is accessible
- Namespace matches Repository location

### API Errors

Common issues:

- **401 Unauthorized**: Check API key validity
- **429 Rate Limited**: Reduce analysis frequency or upgrade plan
- **Timeout**: Increase `timeout_seconds` or reduce context size

Check controller logs:

```bash
kubectl logs -n pipelines-as-code deployment/pipelines-as-code-controller | grep "LLM"
```

### High Costs

To reduce costs:

1. Use more restrictive `on_cel` expressions
2. Lower `max_tokens` value
3. Reduce `container_logs.max_lines`
4. Consider switching to a cheaper model

## Limitations

- Analysis is best-effort and non-blocking
- API key costs are your responsibility
- Subject to LLM provider rate limits
- Context size limited by token constraints
- Not suitable for sensitive/confidential logs

## Further Reading

- [Repository CRD Reference](repositorycrd.md)
- [CEL Expressions Guide](matchingevents.md)
- [Sample Configurations](https://github.com/openshift-pipelines/pipelines-as-code/tree/main/samples)
