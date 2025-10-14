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

- **OpenAI** (GPT-4, GPT-3.5-turbo, etc.)
- **Google Gemini** (gemini-pro, etc.)

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
| `on_cel` | string | No | CEL expression for conditional triggering |
| `output` | string | Yes | Output destination (currently only `pr-comment` is supported) |
| `context_items` | object | No | Configuration for context inclusion |

### Context Items

Control what information is sent to the LLM:

| Field | Type | Description |
|-------|------|-------------|
| `commit_content` | boolean | Include commit message and diff |
| `pr_content` | boolean | Include PR title, description, metadata |
| `error_content` | boolean | Include error messages and failures |
| `container_logs.enabled` | boolean | Include container/task logs |
| `container_logs.max_lines` | integer | Limit log lines (1-1000, default: 50). ⚠️ High values may impact performance |

## CEL Expressions for Triggers

Use CEL expressions in `on_cel` to control when analysis runs:

```yaml
# Only on failures
on_cel: 'body.pipelineRun.status.conditions[0].reason == "Failed"'

# Only on pull requests
on_cel: 'body.event.event_type == "pull_request"'

# Only on main branch
on_cel: 'body.event.base_branch == "main"'

# Combine conditions
on_cel: 'body.pipelineRun.status.conditions[0].reason == "Failed" && body.event.event_type == "pull_request"'
```

Available fields in CEL context:

- `body.pipelineRun` - PipelineRun status and metadata
- `body.event.event_type` - Event type (pull_request, push, etc.)
- `body.event.base_branch` - Target branch name
- `body.event.head_branch` - Source branch name

## Output Destinations

### PR Comment

Posts analysis as a comment on the pull request:

```yaml
output: "pr-comment"
```

Benefits:

- ✅ Visible to all developers
- ✅ Can be updated with new analysis
- ✅ Easy to discuss and follow up

> **Coming Soon**: Additional output destinations including `check-run` (GitHub check runs) and `annotation` (PipelineRun annotations) will be available in future releases.

## Setting Up API Keys

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

1. **Limit max_tokens**: Reduce costs by limiting response length
2. **Use selective triggers**: Only analyze failures, not all runs
3. **Control log lines**: Limit `max_lines` in container logs
4. **Choose efficient models**: Consider using GPT-3.5-turbo for cost savings

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
