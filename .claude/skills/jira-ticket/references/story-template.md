# SRVKP Jira Story Template

Complete template for creating SRVKP Jira stories. Copy this template and fill in the placeholders with actual content.

## Template

```jira
h1. Story (Required)

As a <PERSONA> trying to <ACTION> I want <THIS OUTCOME>

_<Describes high level purpose and goal for this story. Answers the questions: Who is impacted, what is it and why do we need it? How does it improve the customer's experience?>_

h2. *Background (Required)*

_<Describes the context or background related to this story>_

h2. *Out of scope*

_<Defines what is not included in this story>_

h2. *Approach (Required)*

_<Description of the general technical path on how to achieve the goal of the story. Include details like json schema, class definitions>_

h2. *Dependencies*

_<Describes what this story depends on. Dependent Stories and EPICs should be linked to the story.>_

h2. *Acceptance Criteria (Mandatory)*

_<Describe edge cases to consider when implementing the story and defining tests>_

_<Provides a required and minimum list of acceptance tests for this story. More is expected as the engineer implements this story>_

h1. *INVEST Checklist*

Dependencies identified

Blockers noted and expected delivery timelines set

Design is implementable

Acceptance criteria agreed upon

Story estimated

h4. *Legend*

Unknown

Verified

Unsatisfied

h2. *Done Checklist*

* Code is completed, reviewed, documented and checked in

* Unit and integration test automation have been delivered and running cleanly in continuous integration/staging/canary environment

* Continuous Delivery pipeline(s) is able to proceed with new code included

* Customer facing documentation, API docs etc. are produced/updated, reviewed and published

* Acceptance criteria are met
```

## Section-by-Section Guide

### Story (Required)

**Purpose**: Define the user story and high-level goal

**Format**: "As a <PERSONA> trying to <ACTION> I want <THIS OUTCOME>"

**Content**:

1. User story statement (who, what, why)
2. High-level purpose and goal
3. Who is impacted
4. What is it
5. Why do we need it
6. How does it improve customer experience

**Examples**:

```jira
As a platform engineer trying to trigger pipelines from GitLab I want webhook integration with Pipelines-as-Code

Enables automatic pipeline triggering from GitLab repositories when events occur (push, merge request, etc.). This provides GitLab users with the same experience as existing GitHub integration, improving multi-platform support and expanding the user base.
```

```jira
As a developer trying to debug pipeline failures I want structured logging with correlation IDs

Provides better observability into pipeline execution by adding correlation IDs to all log entries. This allows developers to trace a single pipeline run across multiple components and services, reducing debugging time significantly.
```

### Background (Required)

**Purpose**: Provide context and motivation

**Content**:

- Relevant history or context
- Why this is needed now
- Related work or previous attempts
- Problem statement
- User feedback or requests

**Examples**:

```jira
h2. *Background (Required)*

Currently Pipelines-as-Code supports GitHub webhook integration. Multiple customers have requested equivalent functionality for GitLab. This has been the #2 most requested feature in user surveys over the past two quarters. GitLab usage in the organization is growing, with 30% of projects now using GitLab exclusively.
```

```jira
h2. *Background (Required)*

Debugging pipeline failures currently requires searching through logs from multiple pods and components. Engineers report spending 2-3 hours on average tracking down the source of failures. Implementing correlation IDs is a common best practice in microservices architectures and will significantly improve debugging efficiency.
```

### Out of Scope

**Purpose**: Clarify boundaries and prevent scope creep

**Content**:

- What is NOT included
- Future enhancements
- Related but separate features
- Explicit exclusions

**Examples**:

```jira
h2. *Out of scope*

* GitLab CI/CD YAML parsing and conversion
* Migration tools from GitLab CI to Tekton
* Support for self-hosted GitLab instances (planned for future story)
* GitLab Container Registry integration
* GitLab Package Registry integration
```

```jira
h2. *Out of scope*

* Correlation ID propagation to external services (separate story)
* Log aggregation UI changes
* Backwards compatibility for old log format
* Performance testing of new logging implementation
```

### Approach (Required)

**Purpose**: Describe technical implementation path

**Content**:

- General technical approach
- Architecture decisions
- Implementation details
- JSON schemas or data structures
- Class definitions or interfaces
- Technology choices
- Algorithms or patterns

**Examples**:

```jira
h2. *Approach (Required)*

Implement GitLab webhook handler following the existing GitHub webhook architecture:

* Add new endpoint {{/webhook/gitlab}} to webhook server
* Create {{pkg/gitlab}} package with webhook payload parsers
* Implement GitLab signature verification using {{X-Gitlab-Token}} header
* Map GitLab events to Tekton triggers:
  ** Push events → PipelineRun creation
  ** Merge Request events → Preview environment creation
  ** Tag events → Release pipeline triggering
* Reuse existing {{pkg/matcher}} for pipeline matching logic
* Add GitLab-specific configuration to {{Repository}} CRD

JSON schema for GitLab webhook configuration:
{code:json}
{
  "gitlab": {
    "webhookSecret": "string",
    "apiToken": "string",
    "projectId": "number"
  }
}
{code}
```

```jira
h2. *Approach (Required)*

Implement correlation ID middleware for all services:

* Generate UUID v4 at webhook ingestion point
* Propagate via {{X-Correlation-ID}} HTTP header
* Store in request context using {{context.Context}}
* Add to all structured log entries via logger middleware
* Include in error responses for end-to-end tracing

Logger interface modification:
{code:go}
type Logger interface {
    WithCorrelationID(id string) Logger
    Info(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
}
{code}

All log entries will include correlation_id field:
{code:json}
{
  "timestamp": "2024-01-15T10:30:00Z",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "level": "info",
  "message": "Processing webhook"
}
{code}
```

### Dependencies

**Purpose**: Track blockers and related work

**Content**:

- Dependent stories (link in Jira)
- Dependent EPICs (link in Jira)
- External dependencies
- Library or service dependencies
- Upstream/downstream dependencies

**Examples**:

```jira
h2. *Dependencies*

* SRVKP-123: Webhook authentication framework (blocker)
* SRVKP-456: Update Repository CRD schema (dependency)
* EPIC-789: Multi-platform SCM support (parent epic)

External dependencies:
* {{go-gitlab}} v0.95.0+ library for GitLab API
* GitLab instance v15.0+ for webhook support

Blocked by:
* Security review of webhook signature verification (in progress, expected completion: Q1)
```

```jira
h2. *Dependencies*

* SRVKP-234: Structured logging framework (completed)
* SRVKP-567: Logger interface refactoring (in progress)

External dependencies:
* {{go.uber.org/zap}} v1.24.0+ for structured logging
* UUID library for correlation ID generation

No current blockers.
```

### Acceptance Criteria (Mandatory)

**Purpose**: Define success criteria and testing requirements

**Content**:

- Required functionality
- Edge cases to handle
- Performance requirements
- Security requirements
- Minimum acceptance tests
- User-facing criteria
- Non-functional requirements

**Examples**:

```jira
h2. *Acceptance Criteria (Mandatory)*

Functional requirements:
* GitLab push events trigger PipelineRun creation
* Merge request events create preview environments
* Tag push events trigger release pipelines
* Webhook signature verification prevents unauthorized requests
* Invalid payloads return 400 Bad Request with helpful error
* Concurrent webhooks are processed without race conditions

Edge cases:
* Handle webhooks with missing or null fields gracefully
* Support extremely large repository names and file paths
* Process webhooks for empty commits (documentation changes)
* Handle GitLab webhook retries without duplicate runs

Testing requirements:
* Unit tests cover all webhook event types
* Integration tests verify end-to-end webhook processing
* Security tests confirm signature verification
* Load tests demonstrate handling 100 webhooks/second
* E2E tests run against actual GitLab instance

Documentation:
* Setup guide for GitLab webhook configuration
* API documentation for GitLab-specific CRD fields
* Troubleshooting guide for common GitLab webhook issues
```

```jira
h2. *Acceptance Criteria (Mandatory)*

Functional requirements:
* All log entries include correlation_id field
* Correlation ID propagates across service boundaries via HTTP headers
* Same correlation ID used throughout single webhook request lifecycle
* Error responses include correlation ID for debugging
* Correlation ID visible in log aggregation UI

Edge cases:
* Handle requests without incoming correlation ID (generate new one)
* Preserve existing correlation ID when present
* Support correlation ID in both sync and async operations
* Correlation ID included in background job logs

Performance requirements:
* Correlation ID overhead < 1ms per request
* No memory leaks from correlation ID storage
* Log volume increase < 5% (only correlation_id field added)

Testing requirements:
* Unit tests verify correlation ID generation and propagation
* Integration tests trace correlation ID across multiple services
* Performance tests measure overhead
* E2E tests verify end-to-end tracing capability

Documentation:
* Developer guide for using correlation IDs in debugging
* API documentation showing correlation ID in responses
* Runbook for tracing requests using correlation IDs
```

### INVEST Checklist

**Purpose**: Validate story quality using INVEST criteria

**INVEST stands for**:

- **I**ndependent: Can be developed independently
- **N**egotiable: Details can be refined
- **V**aluable: Delivers value to users
- **E**stimable: Can be estimated
- **S**mall: Fits in one sprint
- **T**estable: Has clear acceptance criteria

**Content**: Check off items during story refinement

**Examples**:

```jira
h1. *INVEST Checklist*

Dependencies identified: go-gitlab v0.95.0+, GitLab v15.0+

Blockers noted and expected delivery timelines set: Security review in progress, completion expected by end of Q1

Design is implementable: Yes, reuses existing webhook architecture pattern

Acceptance criteria agreed upon: Reviewed and approved by team

Story estimated: 13 story points
```

```jira
h1. *INVEST Checklist*

Dependencies identified: zap v1.24.0+, UUID library

Blockers noted and expected delivery timelines set: None

Design is implementable: Yes, similar pattern used in other services

Acceptance criteria agreed upon: Pending review with observability team

Story estimated: 8 story points
```

### Done Checklist

**Purpose**: Standard definition of done for all stories

**Content**: Checklist items that must be completed before story is considered done

**Standard items** (do not modify):

- Code is completed, reviewed, documented and checked in
- Unit and integration test automation have been delivered and running cleanly in continuous integration/staging/canary environment
- Continuous Delivery pipeline(s) is able to proceed with new code included
- Customer facing documentation, API docs etc. are produced/updated, reviewed and published
- Acceptance criteria are met

This section should be included as-is in all stories.

## Complete Example: Story for GitLab Webhook Support

```jira
h1. Story (Required)

As a platform engineer trying to trigger pipelines from GitLab I want webhook integration with Pipelines-as-Code

Enables automatic pipeline triggering from GitLab repositories when events occur (push, merge request, tag creation). This provides GitLab users with the same seamless experience as existing GitHub integration, expanding platform support and improving the developer experience for GitLab-based projects.

h2. *Background (Required)*

Currently Pipelines-as-Code supports GitHub webhook integration for automatic pipeline triggering. GitLab is the second most popular Git provider in the organization, with 30% of projects using GitLab exclusively. This feature has been the #2 most requested enhancement in user surveys over the past two quarters.

Without GitLab webhook support, users must either:
* Manually trigger pipelines (poor UX)
* Use polling mechanisms (resource inefficient)
* Maintain separate CI/CD tooling (operational overhead)

Implementing GitLab webhook integration will unify the pipeline experience across Git providers.

h2. *Out of scope*

* GitLab CI/CD YAML parsing and conversion
* Migration tools from GitLab CI to Tekton
* Support for self-hosted GitLab instances (planned for separate story)
* GitLab Container Registry integration
* GitLab Package Registry integration
* GitLab merge train support

h2. *Approach (Required)*

Implement GitLab webhook handler following the existing GitHub webhook architecture:

*Webhook Handler*
* Add new endpoint {{/webhook/gitlab}} to webhook server
* Create {{pkg/gitlab}} package with webhook payload parsers
* Support GitLab event types: Push, Merge Request, Tag Push

*Authentication*
* Implement signature verification using {{X-Gitlab-Token}} header
* Support both webhook tokens and API tokens
* Add GitLab credentials to {{Repository}} CRD

*Event Processing*
* Map GitLab events to Tekton triggers:
  ** Push events → PipelineRun creation
  ** Merge Request events → Preview environment creation
  ** Tag events → Release pipeline triggering
* Reuse existing {{pkg/matcher}} for pipeline matching logic

*Configuration*
GitLab-specific fields in Repository CRD:
{code:yaml}
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: my-gitlab-repo
spec:
  url: "https://gitlab.com/org/repo"
  git_provider:
    type: gitlab
    webhook_secret: gitlab-webhook-secret
    api_token: gitlab-api-token
    project_id: 12345
{code}

*Error Handling*
* Invalid payloads return HTTP 400 with detailed error message
* Signature verification failures return HTTP 401
* Webhook processing errors logged with correlation ID

h2. *Dependencies*

* SRVKP-123: Webhook authentication framework (completed)
* SRVKP-456: Update Repository CRD schema (in progress, blocks implementation)
* EPIC-789: Multi-platform SCM support (parent epic)

External dependencies:
* {{go-gitlab}} v0.95.0+ library for GitLab API
* GitLab instance v15.0+ for webhook v2 API support

Blockers:
* Security review of webhook signature verification (in progress, expected completion: end of Q1)

h2. *Acceptance Criteria (Mandatory)*

*Functional Requirements:*
* GitLab push events trigger PipelineRun creation with correct parameters
* Merge request open/update events create preview environments
* Tag push events trigger release pipelines
* Webhook signature verification prevents unauthorized requests
* Invalid payloads return HTTP 400 with clear error message
* Concurrent webhooks processed without race conditions or duplicate runs

*Edge Cases:*
* Handle webhooks with missing or null optional fields gracefully
* Support repository names and file paths up to 4096 characters
* Process webhooks for empty commits (documentation-only changes)
* Handle GitLab webhook retries without creating duplicate PipelineRuns
* Support webhook delivery even if GitLab API is temporarily unavailable

*Security Requirements:*
* Webhook secret verified on all requests
* API tokens stored securely in Kubernetes secrets
* No sensitive data logged in webhook processing
* Rate limiting prevents webhook flooding attacks

*Performance Requirements:*
* Process webhooks within 500ms (p95)
* Support 100 concurrent webhooks per second
* No memory leaks over 24-hour period

*Testing Requirements:*
* Unit tests cover all GitLab event types (push, merge request, tag)
* Integration tests verify end-to-end webhook processing
* Security tests confirm signature verification and rejection of invalid tokens
* Load tests demonstrate 100 webhooks/second throughput
* E2E tests run against actual GitLab SaaS instance

*Documentation Requirements:*
* User guide for configuring GitLab webhooks
* API documentation for GitLab-specific Repository CRD fields
* Troubleshooting guide for common webhook issues
* Migration guide for users currently using GitLab polling

h1. *INVEST Checklist*

Dependencies identified: go-gitlab v0.95.0+, GitLab v15.0+, Repository CRD update (SRVKP-456)

Blockers noted and expected delivery timelines set: Security review in progress, completion expected end of Q1 2024

Design is implementable: Yes, architecture reuses existing GitHub webhook pattern with minimal modifications

Acceptance criteria agreed upon: Reviewed and approved by team on 2024-01-10

Story estimated: 13 story points

h4. *Legend*

Unknown

Verified

Unsatisfied

h2. *Done Checklist*

* Code is completed, reviewed, documented and checked in

* Unit and integration test automation have been delivered and running cleanly in continuous integration/staging/canary environment

* Continuous Delivery pipeline(s) is able to proceed with new code included

* Customer facing documentation, API docs etc. are produced/updated, reviewed and published

* Acceptance criteria are met
```

## Tips for Writing Good Stories

1. **Be specific in the user story**: Use concrete personas and outcomes
2. **Provide rich background**: Help engineers understand the "why"
3. **Clarify scope**: Out of scope section prevents feature creep
4. **Technical details in approach**: Don't leave implementation ambiguous
5. **Testable acceptance criteria**: Make success measurable
6. **Link dependencies**: Help with sprint planning
7. **Complete INVEST checklist**: Ensures story is ready for work
8. **Use Jira markdown**: Remember h1., h2., *bold*, {{code}}, etc.

## Common Mistakes

❌ **Vague user stories**: "As a user I want a better experience"
✓ **Specific user stories**: "As a platform engineer trying to trigger pipelines from GitLab I want webhook integration"

❌ **Missing technical approach**: "Implement webhook support"
✓ **Detailed approach**: "Add /webhook/gitlab endpoint, implement signature verification using X-Gitlab-Token header, map events to triggers"

❌ **Non-testable criteria**: "System works well"
✓ **Testable criteria**: "Process webhooks within 500ms (p95), support 100 webhooks/second"

❌ **Standard markdown**: `## Background`, `**bold**`
✓ **Jira markdown**: `h2. *Background*`, `*bold*`
