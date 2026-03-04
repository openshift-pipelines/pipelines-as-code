---
title: Metrics
weight: 4
BookToC: false
---

This page describes the Prometheus metrics that Pipelines-as-Code exposes and how to query them. Use these metrics to monitor PipelineRun activity, track Git provider API usage, and observe running workloads.

Pipelines-as-Code serves its metrics through the `pipelines-as-code-watcher` service on port `9090`.

Pipelines-as-Code supports various exporters, such as Prometheus, Google Stackdriver, and more.
You can configure these exporters by referring to the [observability configuration](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/config/305-config-observability.yaml).

## Available Metrics

| Name                                                    | Type       | Labels/Tags                                                                                                                                                                       | Description                                                           |
| ------------------------------------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `pipelines_as_code_git_provider_api_request_count`      | Counter    | `provider`=&lt;git_provider&gt; <br> `event-type`=&lt;event_type&gt; <br> `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                  | Number of API requests submitted to git providers                     |
| `pipelines_as_code_pipelinerun_count`                   | Counter    | `provider`=&lt;git_provider&gt; <br> `event-type`=&lt;event_type&gt; <br> `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                  | Number of PipelineRuns created by Pipelines-as-Code                   |
| `pipelines_as_code_pipelinerun_duration_seconds_sum`    | Counter    | `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt; <br> `status`=&lt;pipelinerun_status&gt; <br> `reason`=&lt;pipelinerun_status_reason&gt;   | Number of seconds all PipelineRuns have taken in Pipelines-as-Code    |
| `pipelines_as_code_running_pipelineruns_count`          | Gauge      | `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                                                                                            | Number of running PipelineRuns in Pipelines-as-Code                   |

{{< callout type="info" >}}
The metric `pipelines_as_code_git_provider_api_request_count`
is emitted by both the Controller and the Watcher, since both services
use Git providers' APIs. When analyzing this metric, you may need to
combine both services' metrics. For example, using PromQL:

- `sum (pac_controller_pipelines_as_code_git_provider_api_request_count or pac_watcher_pipelines_as_code_git_provider_api_request_count)`
- `sum (rate(pac_controller_pipelines_as_code_git_provider_api_request_count[1m]) or rate(pac_watcher_pipelines_as_code_git_provider_api_request_count[1m]))`
{{< /callout >}}

![Prometheus query for git provider API usage metrics combined from both the Watcher and the Controller](/images/git-api-usage-metrics-prometheus-query.png)
