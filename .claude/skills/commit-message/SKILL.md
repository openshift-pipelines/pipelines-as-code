---
name: commit-message
description: This skill should be used when the user asks to "create a commit", "generate commit message", "commit changes", "make a commit", mentions "conventional commits", references "Jira issue in commit", or discusses commit message formatting. Provides guided workflow for creating properly formatted commit messages with Jira integration, line length validation, and required footers.
version: 0.1.0
---

# Conventional Commit Message Creation

Create properly formatted conventional commit messages following project standards with Jira integration, line length validation, and required footers.

## Purpose

Generate commit messages that:

- Follow conventional commits format (`type(scope): description`)
- Integrate Jira issue numbers automatically
- Respect line length limits (50 for subject, 72 for body)
- Include required footers (Signed-off-by, Assisted-by)
- Pass gitlint validation

## Quick Workflow

1. **Analyze changes**: Run git status and git diff to understand modifications
2. **Detect Jira scope**: Extract issue number from branch name or ask user
3. **Generate message**: Create conventional commit message with proper formatting
4. **Add footers**: Include Signed-off-by and Assisted-by trailers
5. **Confirm with user**: Display message and wait for approval before committing

**CRITICAL**: Never commit without explicit user confirmation.

## Conventional Commit Format

### Structure

```text
<type>(<scope>): <description>

[optional body]

Signed-off-by: <name> <email>
Assisted-by: <model-name> (via Claude Code)
```text

### Type Selection

Choose the appropriate commit type based on changes:

| Type | Description | Example |
| ------ | ------------- | --------- |
| `feat` | New features | `feat(webhook): add GitHub App support` |
| `fix` | Bug fixes | `fix(controller): resolve pipeline race condition` |
| `docs` | Documentation | `docs(README): update installation steps` |
| `refactor` | Code refactoring | `refactor(matcher): simplify regex logic` |
| `test` | Test changes | `test(webhook): add integration tests` |
| `chore` | Maintenance | `chore(deps): update go dependencies` |
| `build` | Build system | `build(Makefile): add vendor target` |
| `ci` | CI/CD changes | `ci(github): add golangci-lint action` |
| `perf` | Performance | `perf(cache): optimize lookup speed` |
| `style` | Code style | `style(format): run fumpt formatter` |
| `revert` | Revert commit | `revert: undo breaking API change` |

For complete type reference, see `references/commit-types.md`.

### Scope Rules

#### Priority 1: Extract from branch name

```bash
# Branch: SRVKP-123-add-webhook-support
# Scope: SRVKP-123
# Result: feat(SRVKP-123): add webhook support
```text

Detect Jira issue using regex pattern: `[A-Z]{2,}-[0-9]+`

#### Priority 2: Ask user for Jira issue

If branch is `main`, `master`, or doesn't contain Jira pattern:

1. Ask user for Jira issue number
2. Look up issue on issues.redhat.com if provided
3. Use component name as fallback (webhook, controller, etc.)

#### Priority 3: Component fallback

Use affected component as scope:

- `feat(webhook): ...`
- `fix(controller): ...`
- `docs(README): ...`

## Line Length Requirements

### Subject Line

- **Target**: 50 characters maximum
- **Hard limit**: 72 characters (gitlint enforced)
- **Format**: `type(scope): description` counts toward limit
- **Tips**: Use present tense, no period at end

```text
# Good (45 chars)
feat(SRVKP-123): add webhook controller

# Too long (78 chars) - will fail gitlint
feat(SRVKP-123): add comprehensive webhook controller with GitHub integration
```text

### Body

- **Wrap at 72 characters per line**
- **Blank line** required between subject and body
- **Content**: Explain why, not what (code shows what)
- **Format**: Wrap manually or use heredoc in git commit

```text
feat(SRVKP-123): add webhook controller

Configure webhook controller to handle GitHub events. This enables
real-time pipeline triggering when push events occur, replacing the
previous polling mechanism.

Signed-off-by: Developer Name <developer@redhat.com>
Assisted-by: Claude-3.5-Sonnet (via Claude Code)
```text

## Required Footers

### Signed-off-by

**Always include**: `Signed-off-by: <name> <email>`

**Detection priority order**:

1. Environment variables: `$GIT_AUTHOR_NAME` and `$GIT_AUTHOR_EMAIL`
2. Git config: `git config user.name` and `git config user.email`
3. If neither configured, ask user to provide details

**Common in dev containers**: Environment variables are preferred method

```bash
# Check environment variables first
echo "$GIT_AUTHOR_NAME <$GIT_AUTHOR_EMAIL>"

# Fallback to git config
git config user.name
git config user.email
```text

For complete detection logic, see `references/footer-detection.md`.

### Assisted-by

**Always include**: `Assisted-by: <model-name> (via Claude Code)`

**Format examples**:

```text
Assisted-by: Claude-3.5-Sonnet (via Claude Code)
Assisted-by: Claude Opus 4.5 (via Claude Code)
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

Use the actual model name (Claude Sonnet 4.5, Claude Opus 4.5, etc.).

## User Confirmation Requirement

**CRITICAL RULE**: Always ask for user confirmation before executing `git commit`.

### Confirmation Workflow

1. **Generate** the commit message following all rules above
2. **Display** the complete message to the user with separator
3. **Ask**: "Should I commit with this message? (y/n)"
4. **Wait** for user response
5. **Commit** only if user confirms (yes/y/affirmative)

### Example Interaction

```text
Generated commit message:
---
feat(SRVKP-456): ensure webhook logs output to stdout

Configure webhook controller to direct all logs to stdout for
container compatibility. This resolves logging issues in Kubernetes
environments where logs are collected from stdout.

Signed-off-by: Developer Name <developer@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
---

Should I commit with this message? (y/n)
```text

Wait for user response before proceeding.

## Commit Execution

Use heredoc format for proper multi-line handling:

```bash
git commit -m "$(cat <<'EOF'
feat(SRVKP-456): ensure webhook logs output to stdout

Configure webhook controller to direct all logs to stdout for
container compatibility.

Signed-off-by: Developer Name <developer@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
EOF
)"
```text

**Never use**:

- `--no-verify` (skips pre-commit hooks)
- `--no-gpg-sign` (skips signing)
- `--amend` (unless explicitly requested and safe)

## Complete Examples

### Feature with Jira Scope

```text
feat(SRVKP-789): add GitLab webhook integration

Implement webhook handler for GitLab push events. This enables
pipeline triggering from GitLab repositories using the same
architecture as GitHub integration.

Signed-off-by: Jane Developer <jane@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

### Bug Fix with Component Scope

```text
fix(controller): resolve concurrent pipeline runs

Update pipeline reconciliation logic to handle concurrent runs
correctly. Previously, race condition could cause pipeline state
corruption when multiple runs started simultaneously.

Signed-off-by: John Developer <john@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

### Documentation Update

```text
docs(README): update installation steps

Add section about pre-commit hooks and update Go version
requirement to 1.20.

Signed-off-by: Jane Developer <jane@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

### Breaking Change

```text
feat(api)!: change webhook payload format

Update webhook payload to use standardized event schema. This is a
breaking change requiring webhook consumers to update their
parsers.

BREAKING CHANGE: Webhook payload structure changed from nested
format to flat event schema. See migration guide in docs/.

Signed-off-by: John Developer <john@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```text

## Jira Issue Lookup

When no Jira issue is found in branch name:

1. Ask user: "What Jira issue number should I use for this commit?"
2. If provided, search issues.redhat.com for issue details
3. Use issue summary and description to enhance commit message
4. Suggest creating a new branch with Jira issue in name if desired

For complete Jira lookup workflow, see `references/jira-lookup.md`.

## Gitlint Integration

This project uses gitlint to enforce commit message format. Ensure all commit messages pass gitlint validation.

**Common gitlint rules**:

- Conventional commit format required
- Subject line length limits (50 soft, 72 hard)
- Required footers (Signed-off-by)
- No trailing whitespace
- Body line wrapping at 72 characters

For complete gitlint rules, see `references/gitlint-rules.md`.

## Auto-Detection Summary

When generating commit messages:

1. Run `git status` (without -uall flag)
2. Run `git diff` for staged and unstaged changes
3. Check current branch name for Jira issue pattern `[A-Z]{2,}-[0-9]+`
4. If no Jira issue in branch, ask user for issue number
5. Look up issue details on issues.redhat.com if provided
6. Analyze staged files to determine commit type
7. Generate appropriate scope and description
8. Detect author info from environment variables or git config
9. Ensure subject line is ≤50 characters (max 72)
10. Wrap body text at 72 characters per line
11. Add required footers (Signed-off-by and Assisted-by)
12. Format according to conventional commits standard
13. **Display message and ask for user confirmation**
14. Only commit after receiving confirmation

## Additional Resources

For detailed information:

- **`references/commit-types.md`** - Complete commit type reference with descriptions
- **`references/jira-lookup.md`** - Jira issue lookup workflow and integration
- **`references/gitlint-rules.md`** - Gitlint validation rules and configuration
- **`references/footer-detection.md`** - Author detection logic and priority order
