---
title: API Keys and Endpoints
weight: 2
---

This page explains how to store your LLM provider API keys as Kubernetes secrets and how to configure custom API endpoints. Complete these steps before enabling LLM-powered analysis in your Repository CR.

{{< callout type="warning" >}}
You must create the Secret in the same namespace as the Repository CR.
{{< /callout >}}

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

## Using Custom API Endpoints

By default, Pipelines-as-Code sends requests to each LLM provider's public API. The `api_url` field lets you override the endpoint, which is useful when you run:

- Self-hosted LLM services (for example, LocalAI, vLLM, or Ollama with an OpenAI adapter)
- Enterprise proxy services
- Regional or custom endpoints (for example, Azure OpenAI)
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

If you do not specify `api_url`, Pipelines-as-Code uses these defaults:

- **OpenAI**: `https://api.openai.com/v1`
- **Gemini**: `https://generativelanguage.googleapis.com/v1beta`

### URL Format Requirements

The `api_url` value must:

- Use an `http://` or `https://` scheme
- Include a valid hostname
- Optionally include port and path components

The following examples show valid and invalid formats:

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

## Complete Configuration Example

For a full configuration with multiple roles, see the [complete example](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/samples/repository-llm.yaml) in the Pipelines-as-Code repository.
