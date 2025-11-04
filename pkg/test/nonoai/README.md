# Fake LLM Server for Testing

A lightweight HTTP server that mimics OpenAI and Google Gemini APIs for testing
purposes. This server allows E2E testing of LLM integration without incurring
API costs or depending on external services.

## Features

- ✅ **OpenAI API Compatible** - Supports `/v1/chat/completions` endpoint
- ✅ **Gemini API Compatible** - Supports `/v1beta/models/{model}:generateContent` endpoint
- ✅ **Configurable Responses** - Keyword-based and provider-specific responses
- ✅ **Simulate Failures** - Rate limiting and server errors
- ✅ **Latency Simulation** - Configurable response delays
- ✅ **Health Check** - `/health` endpoint for readiness probes
- ✅ **No Dependencies** - Pure Go implementation
