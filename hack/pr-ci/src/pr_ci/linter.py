"""Lints PR title, description, and commits for conventional format and completeness."""

from __future__ import annotations

import functools
import re
import subprocess
from pathlib import Path
from typing import List, Optional, Tuple

from .comments import PR_TITLE_COMMENT_MARKER, CommentManager
from .gemini import GeminiReleaseNoteChecker
from .github import GitHubClient
from .pr_data import PRData

DEFAULT_JIRA_PROJECT = r"(SRVKP|KONFLUX)"
MIN_DESCRIPTION_LINES = 3


def _get_repo_root() -> Path:
    """Get the repository root directory."""
    # Use git rev-parse to find repo root (works in any environment)
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            check=True,
        )
        return Path(result.stdout.strip())
    except (subprocess.CalledProcessError, FileNotFoundError):
        # Fallback: use current directory
        return Path.cwd()


DEFAULT_PR_TEMPLATE_PATH = _get_repo_root() / ".github" / "pull_request_template.md"
GITHUB_ISSUE_PATTERN = re.compile(
    r"(fixes|closes|resolves)\s+(?:[\w.-]+/[\w.-]+)?#?\d+",
    re.IGNORECASE,
)

JIRA_URL_PATTERN = re.compile(
    rf"https://issues\.redhat\.com/browse/{DEFAULT_JIRA_PROJECT}-\d+",
    re.IGNORECASE,
)

# Release notes section pattern - matches ```release-note block
RELEASE_NOTE_SECTION_PATTERN = re.compile(
    r"#\s*Release\s*Notes",
    re.IGNORECASE,
)

RELEASE_NOTE_BLOCK_PATTERN = re.compile(
    r"```release-note\s*\n(.*?)\n```",
    re.DOTALL | re.IGNORECASE,
)

AI_KEYWORDS = (
    "gemini",
    "chatgpt",
    "gpt",
    "claude",
    "cursor",
    "copilot",
    "codeium",
    "phind",
    "llama",
    "deepseek",
    "tabnine",
    "blackbox",
    "codewhisperer",
)

# Conventional commit types we accept in PR titles
CONVENTIONAL_TYPES: Tuple[str, ...] = (
    "build",
    "chore",
    "ci",
    "docs",
    "deps",
    "enhance",
    "feat",
    "dnm",
    "fix",
    "perf",
    "refactor",
    "release",
    "revert",
    "style",
    "test",
)

# Regex adapted from commitizen default specification
CONVENTIONAL_TITLE_RE = re.compile(
    r"^(?P<type>{types})(\([^)\s]+\))?(?P<breaking>!)?:\s+.+".format(
        types="|".join(CONVENTIONAL_TYPES)
    )
)


class PRLinter:
    """Lints PR title, description, and commits for conventional format and completeness."""

    def __init__(
        self,
        pr_data: PRData,
        github: GitHubClient,
        gemini_api_key: Optional[str] = None,
        gemini_model: Optional[str] = None,
    ):
        self.pr_data = pr_data
        self.github = github
        self.comment_manager = CommentManager(github)
        self.warnings: List[Tuple[str, List[str]]] = []
        self.gemini_api_key = gemini_api_key
        self.gemini_model = gemini_model

    def check_all(self) -> None:
        """Run all lint checks."""
        self.check_title()
        self.check_description()
        self.check_template_usage()
        self.check_jira_reference()
        self.check_ai_attribution()
        self.check_release_notes()

    def check_title(self) -> None:
        """Check if PR title follows conventional commit format."""
        pr_title = self.pr_data.title
        print(f"Linting PR title: {pr_title}")
        is_valid, message = is_conventional_title(pr_title)

        if is_valid:
            print("PR title matches conventional commits format")
        else:
            print(f"PR title validation warning: {message}")
            title_lines = [f"**Current title:** `{pr_title}`"]
            if message:
                title_lines.append("")
                title_lines.append(message)
            title_lines.extend(
                [
                    "",
                    "**Expected pattern:** `<type>(<scope>): <subject>`",
                    "**Allowed types:** " + ", ".join(CONVENTIONAL_TYPES),
                    "",
                    "Examples:",
                    "- `fix(controller): ensure reconciler handles nil spec`",
                    "- `docs: update contributing guide with lint instructions`",
                ]
            )
            self.warnings.append(("PR title format", title_lines))

    def check_description(self) -> None:
        """Check if PR description has enough content."""
        pr_body = self.pr_data.description
        non_empty_lines = [line for line in pr_body.splitlines() if line.strip()]

        if len(non_empty_lines) < MIN_DESCRIPTION_LINES:
            print(
                f"PR description warning: fewer than {MIN_DESCRIPTION_LINES} non-empty lines"
            )
            description_lines = [
                "Please expand the PR description with a brief summary, context, and testing notes.",
                f"Aim for at least {MIN_DESCRIPTION_LINES} meaningful lines so reviewers have enough detail.",
            ]
            self.warnings.append(("PR description completeness", description_lines))

    def check_template_usage(self) -> None:
        """Check if PR description was customized from template."""
        normalized_pr = _sanitize_template_lines(self.pr_data.description)
        default_template = _sanitize_template_lines(get_default_pr_template())

        if default_template:
            custom_lines = [
                line for line in normalized_pr if line not in default_template
            ]
            if not custom_lines:
                print("PR description warning: appears unchanged from template")
                template_lines = [
                    "The PR description still matches the default template.",
                    "Please replace the placeholder sections with project-specific details before review.",
                ]
                self.warnings.append(("PR template usage", template_lines))

    def check_jira_reference(self) -> None:
        """Check if PR has Jira or GitHub issue reference."""
        pr_body = self.pr_data.description
        has_jira = has_required_jira_reference(pr_body)
        has_github = has_github_issue_reference(pr_body)

        if has_jira:
            print("PR description contains a Jira reference")
        elif has_github:
            print("PR description has a GitHub issue reference; skipping Jira reminder")
        else:
            print("PR description warning: missing Jira reference")
            jira_lines = [
                "Add a Jira reference in the description using one of the following formats:",
                "- `https://issues.redhat.com/browse/SRVKP-<number>`",
                "",
                "If no SRVKP ticket exists yet, link a GitHub issue instead (e.g., `Fixes #123`).",
                "Minor housekeeping PRs without Jira coverage can skip this after confirming with reviewers.",
            ]
            self.warnings.append(("Jira reference", jira_lines))

    def check_ai_attribution(self) -> None:
        """Check if commits have AI attribution footers."""
        commits_data = self.github.get_pr_commits()
        missing_ai_commits: List[Tuple[str, str]] = []

        for commit in commits_data:
            sha = (commit.get("sha") or "")[:7]
            message = commit.get("commit", {}).get("message", "")
            summary = message.splitlines()[0] if message else ""

            # Skip merge commits
            if len(commit.get("parents", []) or []) > 1 and summary.startswith("Merge"):
                continue

            if not commit_has_ai_footer(message):
                missing_ai_commits.append((sha, summary))

        if missing_ai_commits:
            print(
                f"AI attribution warning: {len(missing_ai_commits)} commit(s) missing "
                "Assisted-by/Co-authored-by footers"
            )
            ai_lines: List[str] = [
                "The following commits lack an explicit AI attribution footer:",
            ]
            for sha, summary in missing_ai_commits:
                label = f"{sha} `{summary.strip()}`"
                ai_lines.append(f"- {label}")
            ai_lines.extend(
                [
                    "",
                    "If no AI assistance was used for a commit, you can ignore this warning.",
                    "Otherwise add an `Assisted-by:` or `Co-authored-by:` footer referencing the AI used.",
                ]
            )
            self.warnings.append(("AI attribution", ai_lines))
        else:
            print("All commits include AI attribution footers or matching keywords")

    def check_release_notes(self) -> None:
        """Check if PR has a valid release notes section."""
        pr_body = self.pr_data.description
        has_section, has_content, message = has_release_notes(pr_body)

        if has_section and has_content:
            print("PR description contains a valid release notes section")
            return

        # Use AI to check if release notes are required and get suggestions
        is_none_acceptable = False
        ai_reason = ""
        suggested_note = ""
        if self.gemini_api_key:
            print("Using AI to check if release notes are required...")
            ai_result = self._check_release_note_with_ai()
            if ai_result:
                is_none_acceptable = not ai_result.get("required", True)
                ai_reason = ai_result.get("reason", "")
                suggested_note = ai_result.get("suggested_release_note", "")
                if is_none_acceptable:
                    print(f"AI determined release notes are NOT required: {ai_reason}")
                    return
                print(f"AI determined release notes ARE required: {ai_reason}")

        print(f"PR release notes warning: {message}")
        release_notes_lines: List[str] = []

        # Add AI reasoning and suggested note if available
        if ai_reason:
            release_notes_lines.extend(
                [
                    f"**🤖 AI Analysis:** {ai_reason}",
                    "",
                ]
            )

        if suggested_note:
            release_notes_lines.extend(
                [
                    "**💡 Suggested release note:**",
                    "",
                    "    ```release-note",
                    f"    {suggested_note}",
                    "    ```",
                    "",
                ]
            )

        release_notes_lines.extend(
            [
                "Please update the **Release Notes** section in your PR description.",
                "",
                "📍 **Find the section** that looks like this:",
                "",
                "    # Release Notes",
                "    ```release-note",
                "    NONE",
                "    ```",
                "",
                "✏️ **Replace `NONE`** with a brief description of what changed for users.",
                "",
                "**When to use `NONE`:**",
                "- CI/pipeline fixes, test changes, internal refactoring",
                "- Documentation-only updates",
                "- Changes that don't affect how users interact with the product",
            ]
        )
        self.warnings.append(("Release notes", release_notes_lines))

    def _check_release_note_with_ai(self) -> Optional[dict]:
        """Use Gemini AI to check if release notes are required for this PR."""
        if not self.gemini_api_key:
            return None

        try:
            checker = GeminiReleaseNoteChecker(
                self.gemini_api_key,
                self.gemini_model or "gemini-2.0-flash",
            )
            return checker.check_release_note_required(self.pr_data)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"Error checking release notes with AI: {exc}")
            return None

    def report(self) -> None:
        """Post or remove lint feedback comment."""
        existing_comment = self.comment_manager.find_lint_comment()

        if not self.warnings:
            if existing_comment:
                self.comment_manager.delete_comment(existing_comment.get("id"))
                print("Removed previous lint comment since all lint checks now pass")
            return

        # Create a prettier comment with emojis and better formatting
        emoji_map = {
            "PR title format": "📝",
            "PR description completeness": "📄",
            "PR template usage": "📋",
            "Jira reference": "🎫",
            "AI attribution": "🤖",
            "Release notes": "📰",
        }

        comment_lines = [
            PR_TITLE_COMMENT_MARKER,
            "## 🔍 PR Lint Feedback",
            "",
            "> **Note**: This automated check helps ensure your PR follows our contribution guidelines.",
            "",
            "### ⚠️ Items that need attention:",
            "",
        ]

        for heading, lines in self.warnings:
            emoji = emoji_map.get(heading, "⚠️")
            comment_lines.append(f"### {emoji} {heading}")
            comment_lines.append("")

            # Add content in a nice blockquote or code block format
            for line in lines:
                if line.startswith("**") or line.startswith("-"):
                    comment_lines.append(line)
                elif line == "":
                    comment_lines.append("")
                else:
                    comment_lines.append(f"> {line}")
            comment_lines.append("")
            comment_lines.append("---")
            comment_lines.append("")

        # Add footer with helpful info
        comment_lines.extend(
            [
                "### ℹ️ Next Steps",
                "",
                "- Review and address the items above",
                "- Push new commits to update this PR",
                "- This comment will be automatically updated when issues are resolved",
                "",
                "<details>",
                "<summary>🔧 <strong>Admin Tools</strong> (click to expand)</summary>",
                "",
                "**Automated Issue/Ticket Creation:**",
                "",
                "- **`/issue-create`** - Generate a GitHub issue from this PR content using AI",
                "- **`/jira-create`** - Create a SRVKP Jira ticket from this PR content using AI",
                "",
                "> ⚠️ **Important**: Always review and edit generated content before finalizing tickets/issues.",
                "> The AI-generated content should be used as a starting point and may need adjustments.",
                "",
                "*These commands are available to maintainers and will post the generated content as PR comments for review.*",
                "",
                "</details>",
                "",
                "<sub>🤖 *This feedback was generated automatically by the PR CI system*</sub>",
            ]
        )

        body = "\n".join(comment_lines)
        self.comment_manager.upsert_comment(body, existing_comment)


def _sanitize_template_lines(text: str) -> List[str]:
    if not text:
        return []
    without_comments = re.sub(r"<!--.*?-->", "", text, flags=re.DOTALL)
    cleaned_lines: List[str] = []
    for line in without_comments.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        normalized = re.sub(r"\[[xX]\]", "[ ]", stripped)
        cleaned_lines.append(normalized)
    return cleaned_lines


@functools.lru_cache(maxsize=1)
def get_default_pr_template() -> str:
    try:
        return DEFAULT_PR_TEMPLATE_PATH.read_text(encoding="utf-8")
    except OSError as exc:
        print(
            "Warning: Could not read default PR template"
            f" at {DEFAULT_PR_TEMPLATE_PATH}: {exc}"
        )
        return ""


def is_conventional_title(title: str) -> Tuple[bool, Optional[str]]:
    """Validate PR title against the conventional commit format."""
    if not title:
        return False, "PR title is empty"

    if not CONVENTIONAL_TITLE_RE.match(title):
        expected_types = ", ".join(CONVENTIONAL_TYPES)
        return (
            False,
            "Expected format `<type>(<scope>): <subject>` with `<type>` one of "
            f"[{expected_types}].",
        )

    subject = title.split(":", maxsplit=1)[1].strip()
    if len(subject) > 72:
        return False, "Subject should be 72 characters or fewer."

    return True, None


def has_required_jira_reference(description: str) -> bool:
    """Check whether PR description contains a valid Jira reference."""
    if not description:
        return False

    return bool(JIRA_URL_PATTERN.search(description))


def commit_has_ai_footer(message: str) -> bool:
    """Return True if commit message references AI assistance."""
    if not message:
        return False

    lines = [line.strip() for line in message.splitlines() if line.strip()]
    for line in lines:
        lower = line.lower()
        if lower.startswith("assisted-by:") or lower.startswith("ai-assisted-by:"):
            if any(keyword in lower for keyword in AI_KEYWORDS):
                return True
        if lower.startswith("co-authored-by:") and any(
            keyword in lower for keyword in AI_KEYWORDS
        ):
            return True
    return False


def has_github_issue_reference(description: str) -> bool:
    if not description:
        return False
    return bool(GITHUB_ISSUE_PATTERN.search(description))


def has_release_notes(description: str) -> Tuple[bool, bool, str]:
    """Check if PR description contains a valid release notes section.

    Returns:
        Tuple of (has_section, has_content, message):
        - has_section: True if the Release Notes header is present
        - has_content: True if release note block has meaningful content
        - message: Description of any issue found
    """
    if not description:
        return False, False, "PR description is empty"

    # Check for Release Notes section header
    if not RELEASE_NOTE_SECTION_PATTERN.search(description):
        return False, False, "Missing 'Release Notes' section in PR description"

    # Check for release-note code block
    match = RELEASE_NOTE_BLOCK_PATTERN.search(description)
    if not match:
        return True, False, "Missing ```release-note``` block in Release Notes section"

    content = match.group(1).strip()
    if not content:
        return True, False, "Release note block is empty"

    # Check if content is just the default placeholder "NONE"
    if content.upper() == "NONE":
        return (
            True,
            False,
            "Release note is set to 'NONE' - please update with actual release notes "
            "or confirm this PR has no user-facing changes",
        )

    return True, True, "Release notes section is valid"
