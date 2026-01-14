# Jira Markdown Syntax Reference

Complete reference for Jira's text formatting syntax. Jira uses its own markdown syntax that differs significantly from standard markdown.

## Critical Difference

### Jira markdown ≠ Standard markdown

Jira has its own proprietary syntax that predates standard markdown. DO NOT use standard markdown syntax in Jira tickets.

| Feature | Jira Markdown | Standard Markdown |
| --------- | --------------- | ------------------- |
| Heading 1 | `h1. Text` | `# Text` |
| Heading 2 | `h2. Text` | `## Text` |
| Bold | `*text*` | `**text**` or `__text__` |
| Italic | `_text_` | `*text*` or `_text_` |
| Monospace | `{{text}}` | `` `text` `` |
| Link | `[text\|url]` | `[text](url)` |

## Headings

### Syntax

```text
h1. Heading 1
h2. Heading 2
h3. Heading 3
h4. Heading 4
h5. Heading 5
h6. Heading 6
```text

### Rules

- Use `h1.` through `h6.` prefix
- Space required after the period
- No closing syntax needed
- Case sensitive (must be lowercase `h`)

### Examples

✓ **Correct**:

```jira
h1. Main Title
h2. Section Heading
h3. Subsection
```text

**Renders as**:

# Main Title (large)

## Section Heading (medium)

### Subsection (smaller)

❌ **Wrong**:

```text
# Main Title           (standard markdown - won't work)
## Section Heading     (standard markdown - won't work)
H1. Main Title        (uppercase H - won't work)
h1.Main Title         (missing space - won't work)
```text

## Text Formatting

### Bold

**Syntax**: `*text*`

**Examples**:

✓ **Correct**:

```jira
This is *bold text* in the sentence.
*Entire line is bold*
*Multiple* words *can* be *bold*
```text

**Renders as**:
This is **bold text** in the sentence.

❌ **Wrong**:

```text
This is **bold text** in the sentence.  (standard markdown)
This is __bold text__ in the sentence.  (standard markdown)
```text

### Italic

**Syntax**: `_text_`

**Examples**:

✓ **Correct**:

```jira
This is _italic text_ in the sentence.
_Entire line is italic_
_Multiple_ words _can_ be _italic_
```text

**Renders as**:
This is *italic text* in the sentence.

❌ **Wrong**:

```text
This is *italic text* in the sentence.   (standard markdown)
This is /italic text/ in the sentence.   (not supported)
```text

### Bold + Italic

**Syntax**: `*_text_*` or `_*text*_`

**Examples**:

✓ **Correct**:

```jira
This is *_bold and italic_* text.
Also works: _*bold and italic*_ text.
```text

**Renders as**:
This is ***bold and italic*** text.

### Monospace (Code)

**Syntax**: `{{text}}`

**Examples**:

✓ **Correct**:

```jira
Use the {{kubectl}} command.
Set {{DEBUG=true}} in environment.
File path: {{/etc/config/settings.yaml}}
```text

**Renders as**:
Use the `kubectl` command.
Set `DEBUG=true` in environment.
File path: `/etc/config/settings.yaml`

❌ **Wrong**:

```text
Use the `kubectl` command.  (standard markdown backticks)
```text

### Strikethrough

**Syntax**: `-text-`

**Examples**:

✓ **Correct**:

```jira
This is -deleted text- in the sentence.
-Entire line struck through-
```text

**Renders as**:
This is ~~deleted text~~ in the sentence.

### Underline

**Syntax**: `+text+`

**Examples**:

✓ **Correct**:

```jira
This is +underlined text+ in the sentence.
+Entire line underlined+
```text

**Renders as**:
This is <u>underlined text</u> in the sentence.

### Superscript

**Syntax**: `^text^`

**Examples**:

✓ **Correct**:

```jira
E=mc^2^
x^2^ + y^2^ = z^2^
```text

**Renders as**:
E=mc²
x² + y² = z²

### Subscript

**Syntax**: `~text~`

**Examples**:

✓ **Correct**:

```jira
H~2~O is water.
log~10~(100) = 2
```text

**Renders as**:
H₂O is water.
log₁₀(100) = 2

## Lists

### Bulleted Lists

**Syntax**: `*` or `-` at start of line

**Examples**:

✓ **Correct**:

```jira
* Item 1
* Item 2
** Nested item 2.1
** Nested item 2.2
*** Deeply nested 2.2.1
* Item 3

Also works with dashes:
- Item 1
- Item 2
-- Nested item
```text

**Renders as**:

- Item 1
- Item 2
  - Nested item 2.1
  - Nested item 2.2
    - Deeply nested 2.2.1
- Item 3

**Nesting rules**:

- `*` = level 1
- `**` = level 2
- `***` = level 3
- etc.

### Numbered Lists

**Syntax**: `#` at start of line (NOT `1.`)

**Examples**:

✓ **Correct**:

```jira
# First item
# Second item
## Nested item 2.1
## Nested item 2.2
### Deeply nested 2.2.1
# Third item
```text

**Renders as**:

1. First item
2. Second item
   1. Nested item 2.1
   2. Nested item 2.2
      1. Deeply nested 2.2.1
3. Third item

❌ **Wrong**:

```text
1. First item     (standard markdown - won't number)
2. Second item    (standard markdown - won't number)
```text

**Nesting rules**:

- `#` = level 1
- `##` = level 2
- `###` = level 3
- etc.

### Mixed Lists

Can combine bullets and numbers:

```jira
* Bullet item
*# Numbered sub-item
*# Another numbered sub-item
* Another bullet
```text

## Links

### Syntax

**Web links**: `[text | url]` (pipe separator, not parentheses!)

**Jira issues**: `SRVKP-123` or `[SRVKP-123]`

### Examples

✓ **Correct**:

```jira
[GitHub | https://github.com]
[Documentation | https://docs.example.com]
See [Google | https://google.com] for search.
Link to SRVKP-456 or [SRVKP-456]
```text

**Renders as**:

- [GitHub](https://github.com)
- [Documentation](https://docs.example.com)
- See [Google](https://google.com) for search.
- Link to SRVKP-456

❌ **Wrong**:

```text
[GitHub](https://github.com)         (standard markdown parentheses)
[https://github.com](GitHub)         (reversed)
https://github.com                   (bare URL - works but not clickable text)
```text

### Auto-linking

Jira auto-links certain patterns:

- **Issue keys**: `SRVKP-123` → auto-linked
- **URLs**: `https://example.com` → auto-linked
- **Emails**: `user@example.com` → auto-linked

## Code Blocks

### Code with Syntax Highlighting

**Syntax**: `{code:language}...{code}`

**Supported languages**: java, javascript, js, python, go, golang, bash, sh, yaml, yml, json, xml, sql, and many more

**Examples**:

✓ **Correct**:

```jira
{code:go}
func main() {
    fmt.Println("Hello, World!")
}
{code}

{code:bash}
kubectl get pods
kubectl describe pod my-pod
{code}

{code:json}
{
  "name": "example",
  "version": "1.0.0"
}
{code}
```text

❌ **Wrong**:

```text
```go
func main() {}
```text

(standard markdown code fence - won't work)

```text

### Code Without Syntax Highlighting

**Syntax**: `{noformat}...{noformat}`

Use for plain text, logs, or output that shouldn't be syntax highlighted.

**Examples**:

✓ **Correct**:
```jira
{noformat}
$ kubectl logs pod/webhook-abc123
2024-01-15T10:30:00Z INFO Starting server
2024-01-15T10:30:01Z INFO Server listening on :8080
{noformat}

{noformat}
Error: connection refused
  at connect (net.js:123)
  at process._tickCallback (node.js:456)
{noformat}
```text

### Inline vs Block

- **Inline code**: Use `{{text}}` for short code snippets in sentences
- **Block code**: Use `{code}` or `{noformat}` for multi-line code

**Examples**:

```jira
Run {{kubectl get pods}} to list pods.

For detailed output, use:
{code:bash}
kubectl get pods -o yaml
{code}
```text

## Blockquotes

**Syntax**: `bq. Quote text`

**Examples**:

✓ **Correct**:

```jira
bq. This is a quoted text.
bq. Can span multiple lines if needed.

Normal text here.

bq. Another quote.
```text

**Renders as**:
> This is a quoted text.
> Can span multiple lines if needed.

Normal text here.

> Another quote.

❌ **Wrong**:

```text
> This is a quote  (standard markdown - won't work)
```text

## Tables

**Syntax**: ` | |Header 1 | |Header 2 | |` for headers, ` | Cell 1 | Cell 2 | ` for cells

**Examples**:

✓ **Correct**:

```jira
| |Name | |Version | |Status| |
| Pipelines-as-Code | v0.21.0 | Stable |
| Tekton Pipelines | v0.50.0 | Stable |
| Tekton Triggers | v0.24.0 | Beta |
```text

**Renders as**:

| Name | Version | Status |
| ------ | --------- | -------- |
| Pipelines-as-Code | v0.21.0 | Stable |
| Tekton Pipelines | v0.50.0 | Stable |
| Tekton Triggers | v0.24.0 | Beta |

**Rules**:

- Headers: ` | |header | |`
- Cells: ` | cell | `
- Start and end each row with ` | ` or ` | |`
- Cells can contain formatting (`*bold*`, `_italic_`, `{{code}}`, etc.)

**Complex example**:

```jira
| |Feature | |Status | |Priority | |Notes| |
| GitLab webhooks | *In Progress* | {{High}} | SRVKP-123 |
| Bitbucket support | _Planned_ | {{Medium}} | SRVKP-456 |
| Gitea integration | *Completed* | {{Low}} | [SRVKP-789] |
```text

## Horizontal Rule

**Syntax**: `----` (four or more dashes)

**Examples**:

✓ **Correct**:

```jira
Section 1 content

----

Section 2 content
```text

**Renders as**:
Section 1 content

---

Section 2 content

## Panels and Callouts

### Info Panel

**Syntax**: `{info}...{info}`

```jira
{info}
This is informational content.
Useful for tips and notes.
{info}
```text

**Renders as**: Blue panel with info icon

### Warning Panel

**Syntax**: `{warning}...{warning}`

```jira
{warning}
This is a warning.
Use for important cautions.
{warning}
```text

**Renders as**: Yellow panel with warning icon

### Error/Note Panel

**Syntax**: `{note}...{note}`

```jira
{note}
This is a note or error.
Draws attention to important information.
{note}
```text

**Renders as**: Yellow panel with note icon

### Tip Panel

**Syntax**: `{tip}...{tip}`

```jira
{tip}
This is a helpful tip.
Best practices and suggestions.
{tip}
```text

**Renders as**: Green panel with checkmark icon

### Panels with Titles

**Syntax**: `{panel:title=Title Text}...{panel}`

```jira
{panel:title=Important Information}
This is content in a titled panel.
Can include *formatting* and {{code}}.
{panel}
```text

**Renders as**: Panel with custom title

## Colors

**Syntax**: `{color:colorname}text{color}`

**Supported colors**: red, blue, green, yellow, orange, purple, pink, grey/gray, black, white

**Examples**:

```jira
{color:red}This text is red{color}
{color:blue}This text is blue{color}
{color:green}Success message{color}
```text

**Use sparingly**: Colors reduce readability

## Special Characters

### Escaping

Use `\` to escape special characters:

```jira
\* Not a bullet (shows asterisk)
\# Not a numbered list (shows hash)
\{code} Not a code block (shows literal)
```text

### Non-Breaking Space

**Syntax**: Use HTML entity `&nbsp;`

```jira
Word1&nbsp;Word2 (won't break across lines)
```text

## Images

### Attached Images

**Syntax**: `!filename.png!` or `!filename.png | thumbnail!`

```jira
!screenshot.png!
!diagram.png | thumbnail!
!logo.png | width=300!
```text

### External Images

**Syntax**: `!url!`

```jira
!https://example.com/image.png!
!https://example.com/diagram.svg | width=500!
```text

## Mentions and References

### Mention User

**Syntax**: `[~username]` or `[~accountid:123456]`

```jira
Assigned to [~john.developer]
CC: [~jane.engineer]
```text

### Reference Issue

**Syntax**: Issue key (auto-linked)

```jira
Depends on SRVKP-123
Related to OCP-456
Blocks RHCLOUD-789
```text

## Complete Example Document

```jira
h1. Feature Implementation Plan

h2. *Overview*

This document describes the implementation plan for GitLab webhook integration.

See related issue: SRVKP-123

h2. *Background*

{info}
This feature has been requested by 30% of users in recent surveys.
{info}

Currently supported:
* GitHub webhooks
* Bitbucket webhooks

Planned:
# GitLab webhooks (*this feature*)
# Gitea webhooks (SRVKP-456)

h2. *Technical Approach*

The implementation will use the {{go-gitlab}} library:

{code:go}
import (
    "github.com/xanzy/go-gitlab"
)

func HandleGitLabWebhook(payload []byte) error {
    // Implementation here
    return nil
}
{code}

{warning}
Ensure webhook signatures are validated to prevent unauthorized requests.
{warning}

h3. Configuration

Repository CRD will be extended:

{code:yaml}
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
spec:
  git_provider:
    type: gitlab
    webhook_secret: secret-name
    project_id: 12345
{code}

h2. *Test Plan*

| |Test Type | |Status | |Owner| |
| Unit tests | *In Progress* | [~developer1] |
| Integration tests | _Planned_ | [~developer2] |
| E2E tests | _Planned_ | [~qa.engineer] |

h2. *Timeline*

# Design review - _Completed_
# Implementation - *In Progress*
# Testing - Planned
# Documentation - Planned

----

h3. Notes

For more information, see [GitLab Webhook Documentation | https://docs.gitlab.com/ee/user/project/integrations/webhooks.html].

{tip}
Test with a sandbox repository before deploying to production.
{tip}
```text

## Quick Reference Card

| Element | Jira Syntax |
| --------- | ------------- |
| Heading 1 | `h1. Text` |
| Heading 2 | `h2. Text` |
| Bold | `*text*` |
| Italic | `_text_` |
| Monospace | `{{text}}` |
| Strikethrough | `-text-` |
| Underline | `+text+` |
| Bullet list | `* item` |
| Numbered list | `# item` |
| Link | `[text\ | url]` |
| Code block | `{code:lang}...{code}` |
| No format | `{noformat}...{noformat}` |
| Quote | `bq. text` |
| Table header | `\ | \|header\ | \|` |
| Table cell | `\ | cell\ | ` |
| Horizontal rule | `----` |
| Panel | `{info}...{info}` |
| Color | `{color:red}...{color}` |
| Image | `!file.png!` |
| Mention | `[~username]` |

## Common Mistakes

### Wrong: Using Standard Markdown

❌ **Don't do this**:

```markdown
# Heading
## Subheading
**bold** text
`code`
[link](url)
```text

✓ **Do this**:

```jira
h1. Heading
h2. Subheading
*bold* text
{{code}}
[link | url]
```text

### Wrong: Missing Spaces

❌ **Don't do this**:

```jira
h1.Heading          (missing space after period)
*Item               (missing space for bullet)
#Item               (missing space for number)
```text

✓ **Do this**:

```jira
h1. Heading
* Item
# Item
```text

### Wrong: Mixing Syntaxes

❌ **Don't do this**:

```jira
## Heading          (standard markdown)
* *bold* item       (mix of correct and wrong - bullet is right, bold formatting correct but markdown heading wrong)
```text

✓ **Do this**:

```jira
h2. Heading
* *bold* item
```text

## Testing Your Formatting

1. **Preview before submitting**: Use Jira's preview feature
2. **Check special characters**: Ensure they render correctly
3. **Test links**: Click to verify they work
4. **Verify code blocks**: Check syntax highlighting
5. **Review on mobile**: Ensure readability

## Additional Resources

- [Official Jira Text Formatting Notation Help](https://jira.atlassian.com/secure/WikiRendererHelpAction.jspa)
- [Jira Markdown vs Standard Markdown comparison](https://jira.atlassian.com/secure/WikiRendererHelpAction.jspa)

## Summary

Key differences from standard markdown:

- Headings: `h1.` not `#`
- Bold: `*text*` not `**text**`
- Monospace: `{{text}}` not `` `text` ``
- Links: `[text | url]` not `[text](url)`
- Code: `{code}...{code}` not ` ```...``` `
- Numbers: `#` not `1.`

Always use Jira markdown syntax for Jira tickets. Never use standard markdown.
