---
title: Metrics
weight: 16
---

# Metrics Overview

The metrics for pipelines-as-code can be accessed through the `pipelines-as-code-watcher` service on port `9090`.

pipelines-as-code supports various exporters, such as Prometheus, Google Stackdriver, and more.
You can configure these exporters by referring to the [observability configuration](../config/config-observability.yaml).

<<<<<<< HEAD
| Name                                                 | Type    | Description                                                        |
|------------------------------------------------------|---------|--------------------------------------------------------------------|
| `pipelines_as_code_git_provider_api_request_count`   | Counter | Number of API requests submitted to git providers                  |
| `pipelines_as_code_pipelinerun_count`                | Counter | Number of pipelineruns created by pipelines-as-code                |
| `pipelines_as_code_pipelinerun_duration_seconds_sum` | Counter | Number of seconds all pipelineruns have taken in pipelines-as-code |
| `pipelines_as_code_running_pipelineruns_count`       | Gauge   | Number of running pipelineruns in pipelines-as-code                |
=======
| Name                                                    | Type       | Labels/Tags                                                                                                                                                                       | Description                                                           |
| ------------------------------------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `pipelines_as_code_git_provider_api_request_count`      | Counter    | `provider`=&lt;git_provider&gt; <br> `event-type`=&lt;event_type&gt; <br> `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                  | Number of API requests submitted to git providers                     |
| `pipelines_as_code_pipelinerun_count`                   | Counter    | `provider`=&lt;git_provider&gt; <br> `event-type`=&lt;event_type&gt; <br> `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                  | Number of pipelineruns created by pipelines-as-code                   |
| `pipelines_as_code_pipelinerun_duration_seconds_sum`    | Counter    | `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt; <br> `status`=&lt;pipelinerun_status&gt; <br> `reason`=&lt;pipelinerun_status_reason&gt;   | Number of seconds all pipelineruns have taken in pipelines-as-code    |
| `pipelines_as_code_running_pipelineruns_count`          | Gauge      | `namespace`=&lt;pipelinerun_namespace&gt; <br> `repository`=&lt;repository_cr_name&gt;                                                                                            | Number of running pipelineruns in pipelines-as-code                   |
>>>>>>> 5b220d71 (fix: markdownlint error happening due to new version)

**Note:** The metric `pipelines_as_code_git_provider_api_request_count`
is emitted by both the Controller and the Watcher, since both services
use Git providers' APIs. When analyzing this metric, you may need to
combine both services' metrics. For example, using PromQL:

- `sum (pac_controller_pipelines_as_code_git_provider_api_request_count or pac_watcher_pipelines_as_code_git_provider_api_request_count)`
- `sum (rate(pac_controller_pipelines_as_code_git_provider_api_request_count[1m]) or rate(pac_watcher_pipelines_as_code_git_provider_api_request_count[1m]))`

![Prometheus query for git provider API usage metrics combined from both the Watcher and the Controller](/images/git-api-usage-metrics-prometheus-query.png)
