# Jira Issue Lookup Process

Complete workflow for integrating Jira issue numbers into commit messages.

## Overview

Jira issues are used as commit scopes to track work items. This document describes the complete lookup and integration process.

## Auto-Detection from Branch Name

### Detection Pattern

Use regex to extract Jira issue from branch name: `[A-Z]{2,}-[0-9]+`

**Matches**:

- `SRVKP-123-add-webhook-support` → `SRVKP-123`
- `OCP-456-fix-pipeline-bug` → `OCP-456`
- `RHCLOUD-789-implement-feature` → `RHCLOUD-789`

**Does not match**:

- `main` → no Jira issue
- `master` → no Jira issue
- `feature/add-webhook` → no Jira issue (no pattern match)
- `fix-bug` → no Jira issue

### Extraction Process

```bash
# Get current branch name
current_branch=$(git branch --show-current)

# Extract Jira issue using regex
jira_issue=$(echo "$current_branch" | grep -oE '[A-Z]{2,}-[0-9]+' | head -1)

if [ -n "$jira_issue" ]; then
    echo "Found Jira issue: $jira_issue"
    # Use as scope: feat($jira_issue): ...
else
    echo "No Jira issue in branch name"
    # Proceed to manual lookup
fi
```text

### Auto-Detection Examples

**Branch**: `SRVKP-456-ensure-webhook-logs`
**Scope**: `SRVKP-456`
**Result**: `feat(SRVKP-456): ensure webhook logs output to stdout`

**Branch**: `OCP-789-gitlab-integration`
**Scope**: `OCP-789`
**Result**: `feat(OCP-789): add GitLab webhook integration`

## Manual Jira Issue Lookup

When no Jira issue is found in branch name (main, master, or no pattern match):

### Step 1: Ask User for Issue Number

**Prompt**:

```text
No Jira issue found in branch name. What Jira issue number should I use for this commit? (e.g., SRVKP-123, or press Enter to skip)
```text

**Possible responses**:

1. User provides issue number (e.g., `SRVKP-456`)
2. User presses Enter (skip Jira lookup)

### Step 2: Look Up Issue on issues.redhat.com

If user provides issue number, fetch issue details:

**URL format**: `https://issues.redhat.com/browse/{ISSUE_NUMBER}`

**Example**: `https://issues.redhat.com/browse/SRVKP-456`

**Information to extract**:

- **Summary**: Brief description of issue
- **Description**: Detailed context
- **Issue type**: Story, Bug, Task, etc.
- **Status**: Open, In Progress, Resolved, etc.

### Step 3: Enhance Commit Message

Use Jira issue details to improve commit message:

**Issue summary** → Informs commit description
**Issue description** → Provides context for commit body
**Issue type** → Helps select commit type (feat vs fix)

**Example**:

**Jira Issue**: SRVKP-456
**Summary**: "Ensure webhook logs output to stdout"
**Description**: "Currently webhook logs go to file. Need to output to stdout for Kubernetes logging."

**Generated commit**:

```text
feat(SRVKP-456): ensure webhook logs output to stdout

Configure webhook controller to direct all logs to stdout for
container compatibility. This resolves logging issues in Kubernetes
environments where logs are collected from stdout.

Signed-off-by: Developer Name <developer@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

### Step 4: Suggest New Branch Creation

If on main/master, suggest creating a feature branch with Jira issue:

**Prompt**:

```text
You're currently on main. Would you like to create a new branch with the Jira issue in the name? (y/n)

Suggested branch name: SRVKP-456-ensure-webhook-logs-stdout
```text

If user confirms, create branch:

```bash
git checkout -b SRVKP-456-ensure-webhook-logs-stdout
```text

This ensures future commits on this branch auto-detect the Jira issue.

## Component Fallback Scope

If no Jira issue is available or user skips:

### Determine Component from Changed Files

Analyze staged files to identify affected component:

```bash
# Get list of staged files
staged_files=$(git diff --cached --name-only)

# Identify primary component
# Example patterns:
# pkg/webhook/* → webhook
# pkg/controller/* → controller
# pkg/matcher/* → matcher
# docs/* → docs
# test/* → test
```text

### Common Component Scopes

| File pattern | Component scope | Example commit |
| -------------- | ----------------- | ---------------- |
| `pkg/webhook/*` | `webhook` | `feat(webhook): add GitHub App` |
| `pkg/controller/*` | `controller` | `fix(controller): resolve race` |
| `pkg/matcher/*` | `matcher` | `refactor(matcher): simplify` |
| `pkg/apis/*` | `api` | `feat(api): add new endpoint` |
| `docs/*` | `docs` or specific file | `docs(README): update steps` |
| `test/*` | Component being tested | `test(webhook): add tests` |
| `cmd/*` | Command name | `feat(pac): add new flag` |
| Root files | File name | `chore(Makefile): add target` |

### Component Selection Examples

**Changed files**: `pkg/webhook/handler.go`, `pkg/webhook/github.go`
**Component**: `webhook`
**Result**: `feat(webhook): add GitHub App support`

**Changed files**: `docs/README.md`
**Component**: `README`
**Result**: `docs(README): update installation steps`

**Changed files**: Multiple components
**Component**: Most significant or create multiple commits
**Result**: Consider splitting if changes are unrelated

## Jira Issue Validation

### Valid Issue Patterns

**Format**: `{PROJECT}-{NUMBER}`

- **Project**: 2+ uppercase letters
- **Number**: 1+ digits

**Examples**:

- `SRVKP-123` ✓
- `OCP-456` ✓
- `RHCLOUD-7890` ✓
- `srvkp-123` ✗ (lowercase)
- `SRVKP123` ✗ (missing hyphen)
- `S-123` ✗ (project too short)

### Error Handling

If user provides invalid format:

**Prompt**:

```text
Invalid Jira issue format: "srvkp-123"
Expected format: PROJECT-NUMBER (e.g., SRVKP-123)

Please provide a valid Jira issue number, or press Enter to skip:
```text

## Integration with issues.redhat.com

### Web Search Fallback

If direct API access is not available, use web search:

**Query**: `site:issues.redhat.com {ISSUE_NUMBER}`

**Example**: `site:issues.redhat.com SRVKP-456`

Extract issue details from search results and issue page.

### Issue Not Found

If issue doesn't exist or is not accessible:

**Prompt**:

```text
Could not find issue SRVKP-456 on issues.redhat.com.

This might mean:
- Issue doesn't exist
- Issue is not accessible (permissions)
- Issue was moved or deleted

Proceed with commit using SRVKP-456 as scope anyway? (y/n)
```text

If user confirms, use provided issue number despite lookup failure.

## Complete Lookup Workflow

1. **Check branch name** for Jira pattern `[A-Z]{2,}-[0-9]+`
2. If found:
   - Extract issue number
   - Use as scope
   - Optionally look up details for context
3. If not found:
   - Ask user for Jira issue number
   - Validate format
   - Look up on issues.redhat.com
   - Use issue details to enhance commit
   - Suggest creating new branch with issue in name
4. If user skips:
   - Determine component from changed files
   - Use component as scope fallback
5. Generate commit message with determined scope

## Examples

### Example 1: Auto-Detection Success

**Branch**: `SRVKP-789-gitlab-integration`
**Detection**: `SRVKP-789` extracted from branch name
**Lookup**: Optional (for enhancement)
**Result**: `feat(SRVKP-789): add GitLab webhook integration`

### Example 2: Manual Lookup on Main

**Branch**: `main`
**Detection**: No Jira pattern
**User input**: `SRVKP-456`
**Lookup**: Fetch from issues.redhat.com
**Enhancement**: Use issue summary and description
**Result**: `feat(SRVKP-456): ensure webhook logs output to stdout`
**Suggestion**: Create branch `SRVKP-456-ensure-webhook-logs-stdout`

### Example 3: Component Fallback

**Branch**: `main`
**Detection**: No Jira pattern
**User input**: (skipped)
**Changed files**: `pkg/webhook/handler.go`
**Fallback**: `webhook` component
**Result**: `feat(webhook): add request validation`

### Example 4: Multiple Changes

**Branch**: `SRVKP-123-comprehensive-update`
**Detection**: `SRVKP-123`
**Changed files**: Multiple components
**Strategy**: Single commit with umbrella scope, or split into multiple commits
**Result**:

- Option 1: `feat(SRVKP-123): comprehensive webhook updates`
- Option 2: Split into `feat(SRVKP-123): add handler`, `test(SRVKP-123): add tests`

## Best Practices

1. **Prefer auto-detection**: Create branches with Jira issue in name
2. **Look up for context**: Even with auto-detection, fetch issue details to enhance commit message
3. **Validate before committing**: Ensure Jira issue actually exists
4. **Use component fallback wisely**: Only when Jira lookup truly not applicable
5. **Split unrelated changes**: If multiple components, consider multiple commits with same Jira scope
6. **Keep scopes consistent**: Use same scope format throughout project (Jira preferred, component as fallback)
