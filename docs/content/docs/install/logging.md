---
title: Logging
weight: 4
---

## Logging Configuration

Pipelines-as-Code uses a ConfigMap named `pac-config-logging` in its namespace (by default, `pipelines-as-code`) to configure the logging behavior of its controllers.

To view the ConfigMap, use the following command:

```bash
kubectl get configmap pac-config-logging -n pipelines-as-code
```

To see the full content of the ConfigMap, run:

```bash
kubectl get configmap pac-config-logging -n pipelines-as-code -o yaml
```

The `data` section of the ConfigMap contains the following keys:

* `loglevel.pipelinesascode`: The log level for the `pipelines-as-code-controller` component.
* `loglevel.pipelines-as-code-webhook`: The log level for the `pipelines-as-code-webhook` component.
* `loglevel.pac-watcher`: The log level for the `pipelines-as-code-watcher` component.

You can change the log level from `info` to `debug` or any other supported value. For example, to set the log level for the `pipelines-as-code-watcher` to `debug`, run:

```bash
kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "replace", "path": "/data/loglevel.pac-watcher", "value":"debug"}]'
```

The watcher will automatically pick up the new log level.

If you want to use the same log level for all Pipelines-as-Code components, you can remove the individual `loglevel.*` keys. In this case, all components will use the log level defined in the `level` field of the `zap-logger-config`.

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

If you need to troubleshoot interactions with the Git provider API (e.g., GitHub), you can enable detailed API request logging. This is useful for debugging permission issues or unexpected API responses.

To enable this feature, set the log level for the `pipelines-as-code-controller` to `debug`. This will cause the controller to log the duration, URL, and remaining rate-limit for each API call it makes.

You can enable this for the main controller with the following `kubectl` command:

```bash
kubectl patch configmap pac-config-logging -n pipelines-as-code --type json -p '[{"op": "replace", "path": "/data/loglevel.pipelinesascode", "value":"debug"}]'
```

### Rate Limit Monitoring

When debug logging is enabled, Pipelines-as-Code automatically monitors GitHub API rate limits and provides intelligent warnings when limits are running low. This helps prevent API quota exhaustion and provides early warning for potential service disruptions.

The rate limit monitoring includes:

* **Debug level**: All API calls log their duration, URL, and remaining rate limit count
* **Info level**: When remaining calls drop below 500, logs include additional context like total limit and reset time
* **Warning level**: When remaining calls drop below 100, warnings are logged to alert administrators
* **Error level**: When remaining calls drop below 50, critical alerts are logged indicating immediate attention is needed

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

The rate limit information includes:

* **Remaining calls**: Number of API calls left in the current rate limit window
* **Total limit**: The maximum number of calls allowed (typically 5000 for authenticated requests)
* **Reset time**: When the rate limit window resets (shown as Unix timestamp and human-readable time)
* **Repository context**: Which repository triggered the API call (when available)

This monitoring helps administrators:

* **Prevent service disruptions**: Early warnings allow proactive measures before hitting rate limits
* **Optimize API usage**: Identify repositories or operations that consume excessive API calls
* **Plan maintenance windows**: Schedule intensive operations around rate limit reset times
* **Debug authentication issues**: Rate limit headers can indicate token validity and permissions
