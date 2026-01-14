---
name: jira-ticket
description: This skill should be used when the user asks to "create a Jira ticket", "create Jira story", "create Jira bug", "generate Jira issue", "write Jira ticket", mentions "SRVKP ticket", "SRVKP bug", "SRVKP story", or needs help with Jira markdown formatting. Provides templates and formatting guidance for both story and bug report creation.
version: 0.1.0
---

# Jira Ticket Creation

Create properly formatted Jira issues (stories or bugs) using SRVKP templates with correct Jira markdown syntax.

## Purpose

Generate Jira tickets that:

- Follow SRVKP project standards
- Use correct Jira markdown syntax (not standard markdown)
- Include all required sections
- Provide copy-paste ready output for Jira

## When to Use

Use this skill when the user wants to:

- Create a Jira story for new features or enhancements
- Create a Jira bug report for defects or issues
- Get help with Jira markdown formatting
- Generate SRVKP tickets from discussion or context

## Quick Workflow

1. **Determine issue type**: Story or Bug?
2. **Select template**: Use appropriate SRVKP template
3. **Fill in details**: Populate template with user-provided context
4. **Format with Jira markdown**: Ensure correct Jira syntax (not standard markdown!)
5. **Output ready-to-paste**: Provide clean output for Jira text box

## Issue Type Selection

### Story vs Bug Decision

**Create a Story when**:

- Adding new feature or capability
- Enhancing existing functionality
- Implementing new workflow
- User story format applies: "As a X, I want Y, so that Z"

**Create a Bug when**:

- Reporting defect or incorrect behavior
- Fixing crashes or errors
- Resolving incorrect output
- Documenting reproducible issues

### Decision Questions

Ask the user if unclear:

1. "Is this a new feature/enhancement (Story) or a defect/issue (Bug)?"
2. "Does it describe desired new behavior (Story) or incorrect current behavior (Bug)?"
3. "Are you implementing something new (Story) or fixing something broken (Bug)?"

## Jira Markdown Quick Reference

**CRITICAL**: Jira uses its own markdown syntax, NOT standard markdown.

### Essential Jira Markdown

| Element | Jira Markdown | Standard Markdown (DON'T USE) |
| --------- | --------------- | ------------------------------- |
| Heading 1 | `h1. Heading` | `# Heading` |
| Heading 2 | `h2. Heading` | `## Heading` |
| Heading 3 | `h3. Heading` | `### Heading` |
| Bold | `*bold text*` | `**bold text**` |
| Italic | `_italic text_` | `*italic text*` |
| Monospaced | `{{text}}` | `` `text` `` |
| Bulleted list | `* item` or `- item` | `* item` or `- item` ✓ (same) |
| Numbered list | `# item` | `1. item` |
| Code block | `{code:language}...{code}` | ` ```language...``` ` |
| No format | `{noformat}...{noformat}` | N/A |
| Link | `[text\|url]` | `[text](url)` |
| Blockquote | `bq. Quote` | `> Quote` |

### Common Mistakes to Avoid

❌ **Don't use standard markdown**:

```text
## Description     # Wrong!
**bold text**      # Wrong!
`code`             # Wrong!
[link](url)        # Wrong!
```text

✓ **Use Jira markdown**:

```text
h2. Description
*bold text*
{{code}}
[link | url]
```text

For complete Jira markdown syntax, see `references/jira-markdown-syntax.md`.

## Story Template Overview

SRVKP stories follow a structured template with these sections:

### Required Sections

1. **Story (Required)**
   - User story format: "As a X trying to Y, I want Z"
   - High-level purpose and goal
   - Who is impacted, what is it, why needed

2. **Background (Required)**
   - Context or background related to the story
   - Relevant history or motivation

3. **Approach (Required)**
   - Technical path to achieve the goal
   - Implementation details, schemas, class definitions
   - Architecture decisions

4. **Acceptance Criteria (Mandatory)**
   - Required list of acceptance tests
   - Edge cases to consider
   - Definition of done

### Optional Sections

1. **Out of scope**
   - What is NOT included in this story
   - Helps clarify boundaries

2. **Dependencies**
   - What this story depends on
   - Linked stories and epics
   - Blockers

3. **INVEST Checklist**
   - Dependencies identified
   - Blockers noted
   - Design implementable
   - Acceptance criteria agreed
   - Story estimated

4. **Done Checklist**
   - Code completed and reviewed
   - Tests delivered and passing
   - Documentation produced
   - Acceptance criteria met

For complete story template, see `references/story-template.md`.

## Bug Template Overview

SRVKP bug reports follow a structured template with these sections:

### Required Sections

1. **Description of problem**
   - What is broken or incorrect
   - Impact and severity

2. **Prerequisites**
   - Required setup, operators, versions
   - Environment details

3. **Steps to Reproduce**
   - Numbered steps to reproduce the issue
   - Specific and detailed

4. **Actual results**
   - What actually happens (incorrect behavior)

5. **Expected results**
   - What should happen (correct behavior)

6. **Reproducibility**
   - Always / Intermittent / Only Once
   - Helps prioritize and debug

7. **Acceptance criteria**
   - What must be true when bug is fixed
   - Definition of done

### Optional Sections

1. **Workaround**
   - Temporary solution if available

2. **Build Details**
   - Version, commit, build number
   - Environment information

3. **Additional info**
    - Logs, screenshots, stack traces
    - Helpful debugging information

For complete bug template, see `references/bug-template.md`.

## Story Creation Workflow

### Step 1: Gather Information

Ask the user for:

- **What**: What feature or enhancement is needed?
- **Who**: Who is the user/persona?
- **Why**: What problem does this solve?
- **How**: Technical approach or constraints?
- **Acceptance**: How do we know when it's done?

### Step 2: Populate Story Template

Fill in required sections:

```jira
h1. Story (Required)

As a <PERSONA> trying to <ACTION> I want <THIS OUTCOME>

<Describes high level purpose and goal>

h2. *Background (Required)*

<Context and motivation>

h2. *Approach (Required)*

<Technical implementation details>

h2. *Acceptance Criteria (Mandatory)*

* <Acceptance test 1>
* <Acceptance test 2>
* <Acceptance test 3>
```text

### Step 3: Add Optional Sections

Include as appropriate:

- Out of scope (clarify boundaries)
- Dependencies (if any)
- INVEST checklist (for story review)
- Done checklist (standard)

### Step 4: Format with Jira Markdown

Ensure all formatting uses Jira markdown:

- Headings: `h1.`, `h2.`, `h3.`
- Bold: `*text*`
- Italic: `_text_`
- Lists: `*` or `#`
- Code: `{code}...{code}`

### Step 5: Output Ready-to-Paste

Provide clean output that can be copied directly into Jira description field.

## Bug Report Creation Workflow

### Step 1: Gather Information

Ask the user for:

- **Problem**: What is broken or incorrect?
- **Reproduce**: How to trigger the issue?
- **Expect vs Actual**: What should happen vs what happens?
- **Environment**: Versions, setup, prerequisites?
- **Frequency**: Always, intermittent, once?

### Step 2: Populate Bug Template

Fill in required sections:

```jira
h3. *Description of problem:*

<What is broken>

h3. *Prerequisites (if any, like setup, operators/versions):*

<Required environment>

h3. *Steps to Reproduce*

# <step 1>
# <step 2>
# <step 3>

h3. *Actual results:*

<What actually happens>

h3. *Expected results:*

<What should happen>

h3. *Reproducibility (Always/Intermittent/Only Once):*

<Frequency>

h3. *Acceptance criteria:*

<How to verify fix>

*Definition of Done:*
```text

### Step 3: Add Optional Sections

Include as helpful:

- Workaround (if available)
- Build details (version information)
- Additional info (logs, screenshots)

### Step 4: Format with Jira Markdown

Ensure Jira syntax:

- Headings: `h3.` for bug sections
- Bold: `*text*`
- Numbered steps: `#` not `1.`
- Code blocks: `{code}...{code}` or `{noformat}...{noformat}`

### Step 5: Output Ready-to-Paste

Provide clean output for Jira description.

## Examples

### Story Example

**User request**: "Create a Jira story for adding GitLab webhook support to Pipelines-as-Code"

**Output**:

```jira
h1. Story (Required)

As a platform engineer trying to trigger pipelines from GitLab I want webhook integration with Pipelines-as-Code

Enables users to automatically trigger Tekton pipelines when GitLab events occur (push, merge request, etc.). This provides the same experience as existing GitHub integration but for GitLab users.

h2. *Background (Required)*

Currently Pipelines-as-Code supports GitHub webhook integration. GitLab users have requested equivalent functionality to trigger pipelines automatically from their GitLab repositories.

h2. *Out of scope*

* GitLab CI/CD YAML parsing
* Migration tools from GitLab CI
* Self-hosted GitLab instances (future enhancement)

h2. *Approach (Required)*

Implement webhook handler for GitLab events:

* Add GitLab webhook endpoint {{/webhook/gitlab}}
* Parse GitLab webhook payloads
* Map GitLab events to Tekton pipeline triggers
* Implement signature verification for security
* Reuse existing pipeline matching logic

h2. *Dependencies*

* Requires GitLab API library (go-gitlab)
* Depends on SRVKP-XXX for webhook authentication framework

h2. *Acceptance Criteria (Mandatory)*

* GitLab push events trigger pipeline runs
* Merge request events create preview environments
* Webhook signature verification prevents unauthorized requests
* Integration tests cover GitLab webhook scenarios
* Documentation includes GitLab setup guide

h1. *INVEST Checklist*

Dependencies identified: go-gitlab library

Blockers noted: None

Design is implementable: Yes, reuses GitHub webhook architecture

Acceptance criteria agreed upon: Pending review

Story estimated: TBD

h2. *Done Checklist*

* Code is completed, reviewed, documented and checked in
* Unit and integration test automation have been delivered and running cleanly in continuous integration/staging/canary environment
* Continuous Delivery pipeline(s) is able to proceed with new code included
* Customer facing documentation, API docs etc. are produced/updated, reviewed and published
* Acceptance criteria are met
```text

### Bug Example

**User request**: "Create a bug report for webhook controller crashing on invalid JSON payload"

**Output**:

```jira
h3. *Description of problem:*

Webhook controller crashes with nil pointer dereference when receiving invalid JSON payload from GitHub webhooks. This causes webhook processing to fail and requires controller restart.

Workaround: Ensure webhook payloads are valid JSON before sending

h3. *Prerequisites (if any, like setup, operators/versions):*

* Pipelines-as-Code v0.21.0
* Tekton Pipelines v0.50.0
* GitHub webhook configured

h3. *Steps to Reproduce*

# Configure GitHub webhook pointing to Pipelines-as-Code
# Send malformed JSON payload to webhook endpoint
# Observe controller logs

h3. *Actual results:*

Controller crashes with error:
{noformat}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]
{noformat}

Webhook processing stops. Controller restart required.

h3. *Expected results:*

Controller should handle invalid JSON gracefully:
* Log error message with payload details
* Return HTTP 400 Bad Request to GitHub
* Continue processing other webhooks normally
* No crash or restart required

h3. *Reproducibility (Always/Intermittent/Only Once):*

Always - occurs every time invalid JSON is sent

h3. *Acceptance criteria:*

* Invalid JSON payloads do not crash controller
* Error is logged with helpful message
* HTTP 400 response returned to webhook sender
* Unit test covers invalid JSON handling
* Controller remains operational after invalid payload

*Definition of Done:*

* Bug fix merged and deployed
* Test coverage demonstrates fix
* No controller crashes on invalid JSON

h3. *Build Details:*

* Version: v0.21.0
* Commit: abc123def456
* Environment: OpenShift 4.13

h3. *Additional info (Such as Logs, Screenshots, etc):*

Full stack trace:
{code}
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x1234567]

goroutine 42 [running]:
pkg/webhook.parsePayload(0xc000123456)
    webhook/handler.go:123
{code}
```text

## Output Format Guidelines

### Structure

1. **Clean formatting**: Use proper Jira markdown throughout
2. **No markdown artifacts**: Remove any code fences, standard markdown
3. **Copy-paste ready**: User should copy entire output to Jira

### Presentation

Present the final output with clear instructions:

```text
Here's your Jira ticket. Copy the entire content below and paste it into the Description field when creating the Jira issue:

---

[TICKET CONTENT HERE]

---

Remember to:
1. Select the correct issue type (Story/Bug)
2. Fill in the Summary field separately
3. Paste this content into the Description field
4. Assign to appropriate sprint/epic
```text

### Validation

Before outputting:

- ✓ All headings use `h1.`, `h2.`, `h3.` format
- ✓ Bold uses `*text*` not `**text**`
- ✓ Monospace uses `{{text}}` not `` `text` ``
- ✓ Code blocks use `{code}...{code}` not ` ```...``` `
- ✓ Links use `[text | url]` not `[text](url)`
- ✓ Lists use `#` for numbered, `*` for bullets
- ✓ No standard markdown syntax

## Additional Resources

For detailed information:

- **`references/story-template.md`** - Complete SRVKP story template with all sections
- **`references/bug-template.md`** - Complete SRVKP bug report template with all sections
- **`references/jira-markdown-syntax.md`** - Comprehensive Jira markdown reference

## Common Questions

**Q: Can I use standard markdown?**
A: No! Jira has its own syntax. Use `h2.` not `##`, `*bold*` not `**bold**`, etc.

**Q: What's the difference between Story and Bug?**
A: Story = new feature/enhancement. Bug = defect/incorrect behavior.

**Q: Do I need to fill every section?**
A: Required sections are mandatory. Optional sections should be included when relevant.

**Q: Can I modify the templates?**
A: The SRVKP templates are standardized. Fill them as-is for consistency across the project.

**Q: What if I don't have all the information?**
A: Fill in what you know and mark missing sections with "TBD" or ask the user for clarification.

**Q: How do I add links in Jira?**
A: Use `[link text | https://url]` format, not standard markdown `[text](url)`.

**Q: What about code examples in tickets?**
A: Use `{code:language}...{code}` blocks for code. Example: `{code:go}...{code}` or `{code:bash}...{code}`.

## Best Practices

1. **Ask clarifying questions**: Don't guess missing information
2. **Use correct issue type**: Story for features, Bug for defects
3. **Be specific**: Provide concrete examples and details
4. **Include acceptance criteria**: Make success measurable
5. **Use Jira markdown**: Never standard markdown
6. **Copy-paste ready**: Output should work directly in Jira
7. **Validate syntax**: Check all formatting before outputting
8. **Reference templates**: Use complete templates from references/ when needed
