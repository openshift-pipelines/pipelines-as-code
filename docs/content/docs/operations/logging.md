---
title: Logging
weight: 3
---

This page explains how to configure logging levels for Pipelines-as-Code components and how to enable detailed API debugging. Adjust these settings when you need to troubleshoot failures, investigate permission issues, or monitor API rate limits.

## Logging Configuration

Pipelines-as-Code stores its logging configuration in a ConfigMap named `pac-config-logging` in the `pipelines-as-code` namespace. This ConfigMap controls the verbosity of each controller component.

To view the ConfigMap:

```bash
kubectl get configmap pac-config-logging -n pipelines-as-code
```

To see the full content:

```bash
kubectl get configmap pac-config-logging -n pipelines-as-code -o yaml
```

The `data` section contains the following keys:

* `loglevel.pipelinesascode`: The log level for the `pipelines-as-code-controller` component.
* `loglevel.pipelines-as-code-webhook`: The log level for the `pipelines-as-code-webhook` component.
* `loglevel.pac-watcher`: The log level for the `pipelines-as-code-watcher` component.

You can change any log level from `info` to `debug` or any other supported value. For example, to set the watcher log level to `debug`:

```bash
kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "replace", "path": "/data/loglevel.pac-watcher", "value":"debug"}]'
```

The watcher picks up the new log level automatically without a restart.

If you want all Pipelines-as-Code components to share the same log level, remove the individual `loglevel.*` keys. All components then fall back to the `level` field defined in `zap-logger-config`:

```bash
kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "remove", "path": "/data/loglevel.pac-watcher"}, {"op": "remove", "path": "/data/loglevel.pipelines-as-code-webhook"}, {"op": "remove", "path": "/data/loglevel.pipelinesascode"}]'
```

The `zap-logger-config` supports the following log levels:

* `debug`: Fine-grained debugging information.
* `info`: Normal operational logging.
* `warn`: Unexpected but non-critical errors.
* `error`: Critical errors that are unexpected during normal operation.
* `dpanic`: Triggers a panic (crash) in development mode.
* `panic`: Triggers a panic (crash).
* `fatal`: Immediately exits with a status of 1.

For more details, see the [Knative logging documentation](https://knative.dev/docs/serving/observability/logging/config-logging).

## Debugging API Interactions

When you need to troubleshoot interactions with a Git provider API (e.g., GitHub), you can enable detailed request logging. This is useful for diagnosing permission issues or unexpected API responses.

To enable this feature, set the controller log level to `debug`. Pipelines-as-Code then logs the duration, URL, and remaining rate limit for each API call:

```bash
kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "replace", "path": "/data/loglevel.pipelinesascode", "value":"debug"}]'
```

### Rate Limit Monitoring

With debug logging enabled, Pipelines-as-Code automatically monitors GitHub API rate limits and warns you when limits run low. This helps you take action before API quota exhaustion causes service disruptions.

The monitoring reports at different severity levels depending on the remaining quota:

* **Debug level**: Every API call logs its duration, URL, and remaining rate limit count.
* **Info level**: When remaining calls drop below 500, logs include additional context such as total limit and reset time.
* **Warning level**: When remaining calls drop below 100, Pipelines-as-Code logs warnings to alert administrators.
* **Error level**: When remaining calls drop below 50, Pipelines-as-Code logs critical alerts indicating immediate attention is needed.

#### Example Log Messages

```console
# Debug level - normal API call logging
{"level":"debug","ts":"...","caller":"...","msg":"GitHub API call completed","operation":"get_pull_request","duration_ms":23,"provider":"github","repo":"myorg/myrepo","url_path":"/repos/myorg/myrepo/pulls/1","rate_limit_remaining":"4850","status_code":200}
# Info level - moderate rate limit usage
{"level":"info","ts":"...","caller":"...","msg":"GitHub API rate limit moderate","repo":"myorg/myrepo","remaining":350,"limit":"5000","reset":"1672531200 (15:30:00 UTC)"}
# Warning level - low rate limit
{"level":"warn","ts":"...","caller":"...","msg":"GitHub API rate limit running low","repo":"myorg/myrepo","remaining":75,"limit":"5000","reset":"1672531200 (15:30:00 UTC)"}
# Error level - critically low rate limit
{"level":"error","ts":"...","caller":"...","msg":"GitHub API rate limit critically low","repo":"myorg/myrepo","remaining":25,"limit":"5000","reset":"1672531200 (15:30:00 UTC)"}
```

Each rate limit log entry includes:

* **Remaining calls**: The number of API calls left in the current rate limit window.
* **Total limit**: The maximum number of calls allowed (typically 5000 for authenticated requests).
* **Reset time**: When the rate limit window resets (shown as Unix timestamp and human-readable time).
* **Repository context**: Which repository triggered the API call (when available).

This monitoring helps you:

* **Prevent service disruptions**: Act on early warnings before you hit rate limits.
* **Optimize API usage**: Identify repositories or operations that consume excessive API calls.
* **Plan maintenance windows**: Schedule intensive operations around rate limit reset times.
* **Debug authentication issues**: Rate limit headers can indicate token validity and permissions.
