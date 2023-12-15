---
title: Concurrency Flow
weight: 2
---
# Concurrency Flow

{{< mermaid >}}

graph TD
    A1[Controller] --> B1(Validate & Process Event)
    B1 --> C1{Is concurrency defined?}
    C1 -->|Not Defined| D1[Create PipelineRun with state='started']
    C1 -->|Defined| E1[Create PipelineRun with pending status and state='queued']

    Z[Pipelines-as-Code]

    A[Watcher] --> B(PipelineRun Reconciler)
    B --> C{Check state}
    C --> |completed| F(Return, nothing to do!)
    C --> |queued| D(Create Queue for Repository)
    C --> |started| E{Is PipelineRun Done?}
    D --> O(Add PipelineRun in the queue)
    O --> P{If PipelineRuns running < concurrency_limit}
    P --> |Yes| Q(Start the top most PipelineRun in the Queue)
    Q --> P
    P --> |No| R[Return and wait for your turn]
    E --> |Yes| G(Report Status to provider)
    E --> |No| H(Requeue Request)
    H --> B
    G --> I(Update status in Repository)
    I --> J(Update state to 'completed')
    J --> K{Check if concurrency was defined?}
    K --> |Yes| L(Remove PipelineRun from Queue)
    L --> M(Start the next PipelineRun from Queue)
    M --> N[Done!]
    K --> |No| N

{{< /mermaid >}}
