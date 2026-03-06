---
title: "Repository Status"
weight: 3
---

This page documents the status fields that Pipelines-as-Code populates on each Repository CR. Use this reference to understand what information you can query about past PipelineRun executions. The `pipelinerun_status` field contains an array of `RepositoryRunStatus` objects, with the most recent runs typically appearing last.

## RepositoryRunStatus

Each entry represents the outcome of a single PipelineRun execution.

{{< param name="pipelineRunName" type="string" id="param-pipeline-run-name" >}}
Identifies the PipelineRun resource by name.

```yaml
pipelinerun_status:
  - pipelineRunName: "my-repo-run-abc123"
```

{{< /param >}}

{{< param name="startTime" type="metav1.Time" id="param-start-time" >}}
Records when the PipelineRun started execution. Uses Kubernetes time format (RFC3339).

```yaml
pipelinerun_status:
  - startTime: "2024-03-01T10:30:00Z"
```

{{< /param >}}

{{< param name="completionTime" type="metav1.Time" id="param-completion-time" >}}
Records when the PipelineRun completed execution. Uses Kubernetes time format (RFC3339).

```yaml
pipelinerun_status:
  - completionTime: "2024-03-01T10:45:00Z"
```

{{< /param >}}

{{< param name="sha" type="string" >}}
Contains the Git commit SHA that Pipelines-as-Code tested in this PipelineRun.

```yaml
pipelinerun_status:
  - sha: "abc123def456"
```

{{< /param >}}

{{< param name="sha_url" type="string" id="param-sha-url" >}}
Provides the URL to view the commit SHA in the Git provider UI.

```yaml
pipelinerun_status:
  - sha_url: "https://github.com/owner/repo/commit/abc123"
```

{{< /param >}}

{{< param name="title" type="string" >}}
Contains the title of the tested commit (typically the first line of the commit message).

```yaml
pipelinerun_status:
  - title: "Fix authentication bug"
```

{{< /param >}}

{{< param name="logurl" type="string" >}}
Provides the full URL to the logs for this PipelineRun.

```yaml
pipelinerun_status:
  - logurl: "https://tekton.example.com/logs/my-run"
```

{{< /param >}}

{{< param name="target_branch" type="string" id="param-target-branch" >}}
Identifies the target branch for this run (for example, the base branch of a pull request).

```yaml
pipelinerun_status:
  - target_branch: "main"
```

{{< /param >}}

{{< param name="event_type" type="string" id="param-event-type" >}}
Identifies the event type that triggered this run (for example, `push` or `pull_request`).

```yaml
pipelinerun_status:
  - event_type: "pull_request"
```

{{< /param >}}

{{< param name="conditions" type="[]Condition" >}}
Describes the current state of the PipelineRun. Follows the standard Knative Conditions pattern.

{{< param-group label="Show Condition Fields" >}}

{{< param name="type" type="string" >}}
Identifies the condition type. Typically `Succeeded` for PipelineRun completion status.
{{< /param >}}

{{< param name="status" type="string" >}}
Indicates the condition status. Can be `True`, `False`, or `Unknown`.
{{< /param >}}

{{< param name="reason" type="string" >}}
Provides a machine-readable reason for the condition status.
{{< /param >}}

{{< param name="message" type="string" >}}
Provides a human-readable message with details about the condition.
{{< /param >}}

{{< /param-group >}}

```yaml
pipelinerun_status:
  - conditions:
      - type: Succeeded
        status: "True"
        reason: Succeeded
        message: "All tasks completed successfully"
```

{{< /param >}}

{{< param name="failure_reason" type="map[string]TaskInfos" id="param-failure-reason" >}}
Contains information about individual tasks, particularly useful for diagnosing failures. Maps task names to their failure details.

{{< param-group label="Show TaskInfos Fields" >}}

{{< param name="name" type="string" >}}
Identifies the task by name.
{{< /param >}}

{{< param name="message" type="string" >}}
Contains the error or status message from the task.
{{< /param >}}

{{< param name="log_snippet" type="string" id="param-log-snippet" >}}
Contains a snippet of logs from the failed task.
{{< /param >}}

{{< param name="reason" type="string" >}}
Provides the reason for task failure.
{{< /param >}}

{{< param name="display_name" type="string" id="param-display-name" >}}
Provides a human-friendly display name for the task.
{{< /param >}}

{{< param name="completion_time" type="metav1.Time" id="param-task-completion-time" >}}
Records when the task completed.
{{< /param >}}

{{< /param-group >}}

```yaml
pipelinerun_status:
  - failure_reason:
      build-task:
        name: "build-task"
        message: "Build failed: compilation error"
        log_snippet: "error: undefined reference to 'main'"
        reason: "BuildFailed"
        display_name: "Build Application"
        completion_time: "2024-03-01T10:42:00Z"
```

{{< /param >}}

## Complete example

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: example-repo
  namespace: pipelines-as-code
spec:
  url: "https://github.com/organization/repository"
pipelinerun_status:
  - pipelineRunName: "example-repo-run-xyz789"
    startTime: "2024-03-01T10:30:00Z"
    completionTime: "2024-03-01T10:45:00Z"
    sha: "abc123def456789"
    sha_url: "https://github.com/organization/repository/commit/abc123def456789"
    title: "Add new feature for user authentication"
    logurl: "https://tekton.example.com/namespaces/pipelines-as-code/pipelineruns/example-repo-run-xyz789"
    target_branch: "main"
    event_type: "pull_request"
    conditions:
      - type: Succeeded
        status: "True"
        reason: Succeeded
        message: "Tasks Completed: 3 (Failed: 0, Cancelled: 0), Skipped: 0"
        lastTransitionTime: "2024-03-01T10:45:00Z"
  - pipelineRunName: "example-repo-run-abc456"
    startTime: "2024-03-01T09:00:00Z"
    completionTime: "2024-03-01T09:10:00Z"
    sha: "def456abc789012"
    sha_url: "https://github.com/organization/repository/commit/def456abc789012"
    title: "Fix build script"
    logurl: "https://tekton.example.com/namespaces/pipelines-as-code/pipelineruns/example-repo-run-abc456"
    target_branch: "main"
    event_type: "push"
    conditions:
      - type: Succeeded
        status: "False"
        reason: Failed
        message: "TaskRun example-repo-run-abc456-build failed"
        lastTransitionTime: "2024-03-01T09:10:00Z"
    failure_reason:
      build:
        name: "build"
        message: "Build script failed with exit code 1"
        log_snippet: |
          + npm run build
          ERROR: Module not found
          npm ERR! code 1
        reason: "BuildScriptFailed"
        display_name: "Build Application"
        completion_time: "2024-03-01T09:10:00Z"
```

## Accessing status

You can query the status using `kubectl`:

```bash
# Get the full status
kubectl get repository example-repo -o jsonpath='{.pipelinerun_status}' | jq .

# Get the latest run status
kubectl get repository example-repo -o jsonpath='{.pipelinerun_status[-1]}' | jq .

# Get the last run's success status
kubectl get repository example-repo -o jsonpath='{.pipelinerun_status[-1].conditions[?(@.type=="Succeeded")].status}'
```
