"""Analyzes PR content using Gemini AI for labels and release notes."""

from __future__ import annotations

import json
from typing import List, Optional

import google.generativeai as genai

from .config import DEFAULT_MODEL
from .pr_data import PRData


class GeminiPRAnalyzer:
    """Unified analyzer for PR labels and release notes using Gemini AI."""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        genai.configure(api_key=api_key)
        self.model = genai.GenerativeModel(model_name)
        self.model_name = model_name

    def analyze_pr(
        self,
        pr_data: PRData,
        available_labels: Optional[List[dict]] = None,
        excluded_labels: Optional[set[str]] = None,
    ) -> dict:
        """Analyze PR and return labels, release note requirement, and suggested note.

        Returns:
            dict with keys:
            - labels: List[str] - suggested labels (empty if no available_labels provided)
            - release_note_required: bool - whether release notes are required
            - release_note_reason: str - explanation for the decision
            - suggested_release_note: str - suggested release note text (if required)
        """
        try:
            prompt = self._build_prompt(pr_data, available_labels, excluded_labels)
            response = self.model.generate_content(prompt)
            return self._parse_response(response)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"Error with Gemini API: {exc}")
            return {
                "labels": [],
                "release_note_required": True,
                "release_note_reason": "Could not analyze PR (defaulting to required)",
                "suggested_release_note": "",
            }

    def _build_prompt(
        self,
        pr_data: PRData,
        available_labels: Optional[List[dict]] = None,
        excluded_labels: Optional[set[str]] = None,
    ) -> str:
        """Build the combined prompt for Gemini."""
        commits_text = "\n".join([f"- {msg}" for msg in pr_data.commit_messages])
        files_text = "\n".join(pr_data.files_changed)

        # Format labels section if available
        labels_section = ""
        if available_labels:
            excluded = excluded_labels or set()
            labels_with_descriptions: List[str] = []
            for label in available_labels:
                if label["name"] in excluded:
                    continue
                if label.get("description"):
                    labels_with_descriptions.append(
                        f"{label['name']}: {label['description']}"
                    )
                else:
                    labels_with_descriptions.append(label["name"])
            labels_text_formatted = "\n".join(labels_with_descriptions)

            labels_section = f"""
## TASK 1: Suggest Labels

AVAILABLE LABELS (only suggest from this list):
{labels_text_formatted}

LABEL RULES:
- Maximum 3 labels
- Only "documentation" if docs/ files modified
- Only "e2e" if test/ files modified for e2e tests
- Only provider labels (github, gitlab, bitbucket, gitea) if pkg/provider/<provider>/ modified
"""

        return f"""
You are analyzing a Pull Request for "Pipelines as Code" (a Tekton-based CI/CD system).

PR Title: {pr_data.title}

PR Description:
{pr_data.description}

Files changed:
{files_text}

Commit messages:
{commits_text}

---
{labels_section}
## TASK 2: Determine Release Note Requirement

Release notes are ONLY for changes that END USERS need to know about.

IMPORTANT: Default to NOT requiring release notes unless the change clearly affects end users.

NO RELEASE NOTES NEEDED (release_note_required: false):
- CI/CD pipelines, GitHub Actions, Tekton tasks (.tekton/, .github/workflows/)
- Tests (test/, *_test.go, *_test.py)
- Development tooling (hack/, scripts/, Makefile, pre-commit)
- Linters, formatters, code quality tools
- Internal refactoring without behavior change
- Internal logging/error messages
- Documentation updates (docs/, README)
- Dependency updates (unless fixing user-facing bugs)
- Build system, developer experience improvements
- Flaky test fixes, code cleanup

RELEASE NOTES REQUIRED (release_note_required: true):
- New user-facing features
- Bug fixes users would notice
- CLI command/flag changes
- Kubernetes CRD/API changes
- Configuration option changes
- Security fixes
- Breaking changes
- Webhook handling changes affecting users
- Pipeline trigger/run behavior changes

FILE PATH HINTS:
- Mostly in test/, hack/, .tekton/, .github/ → NO release notes
- In pkg/ but only logging/refactoring → NO release notes
- User-visible behavior in pkg/, cmd/ → MAYBE release notes

Be conservative: when in doubt, release_note_required: false.

---

## TASK 3: Suggest Release Note (only if required)

If release_note_required is true, write a concise release note (1-2 sentences) describing:
- What changed from the USER's perspective
- Any action users need to take

---

Respond with ONLY valid JSON:
{{
  "labels": ["label1", "label2"],
  "release_note_required": true/false,
  "release_note_reason": "Brief explanation",
  "suggested_release_note": "Release note text if required, empty string if not"
}}
"""

    def _parse_response(self, response) -> dict:
        """Parse Gemini response."""
        default_result = {
            "labels": [],
            "release_note_required": True,
            "release_note_reason": "Could not parse response",
            "suggested_release_note": "",
        }

        try:
            response_text = response.text.strip()
            if response_text.startswith("```json"):
                response_text = response_text[7:-3]
            elif response_text.startswith("```"):
                response_text = response_text[3:-3]

            result = json.loads(response_text)
            if not isinstance(result, dict):
                return default_result

            return {
                "labels": result.get("labels", []) or [],
                "release_note_required": bool(
                    result.get("release_note_required", True)
                ),
                "release_note_reason": result.get("release_note_reason", "No reason"),
                "suggested_release_note": result.get("suggested_release_note", "")
                or "",
            }
        except json.JSONDecodeError as exc:
            print(f"Could not parse Gemini response as JSON: {exc}")
            return default_result


# Backward compatibility wrappers
class GeminiAnalyzer:
    """Analyzes PR content using Gemini AI to suggest labels. (Legacy wrapper)"""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        self._analyzer = GeminiPRAnalyzer(api_key, model_name)

    def suggest_labels(
        self,
        pr_data: PRData,
        available_labels: List[dict],
        excluded_labels: set[str],
    ) -> List[str]:
        """Analyze PR and suggest appropriate labels."""
        result = self._analyzer.analyze_pr(pr_data, available_labels, excluded_labels)
        return result.get("labels", [])


class GeminiReleaseNoteChecker:
    """Checks if a PR requires release notes. (Legacy wrapper)"""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        self._analyzer = GeminiPRAnalyzer(api_key, model_name)

    def check_release_note_required(self, pr_data: PRData) -> dict:
        """Analyze PR and determine if release notes are required."""
        result = self._analyzer.analyze_pr(pr_data)
        return {
            "required": result.get("release_note_required", True),
            "reason": result.get("release_note_reason", ""),
            "suggested_release_note": result.get("suggested_release_note", ""),
        }


class GeminiIssueGenerator:
    """Generates GitHub issue content using Gemini AI."""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        genai.configure(api_key=api_key)
        self.model = genai.GenerativeModel(model_name)
        self.model_name = model_name

    def generate_issue(self, pr_data: PRData) -> dict:
        """Analyze PR and generate GitHub issue content."""
        try:
            prompt = self._build_prompt(pr_data)
            response = self.model.generate_content(prompt)
            return self._parse_response(response)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"Error with Gemini API: {exc}")
            return {}

    def _build_prompt(self, pr_data: PRData) -> str:
        """Build the prompt for Gemini."""
        commits_text = "\n".join([f"- {msg}" for msg in pr_data.commit_messages])
        files_text = "\n".join(pr_data.files_changed)

        return f"""
Analyze this GitHub Pull Request and generate a GitHub issue that describes the PROBLEM that this PR solves, not the solution itself.

PR Title: {pr_data.title}

PR Description:
{pr_data.description}

Files changed:
{files_text}

Commit messages:
{commits_text}

Based on the PR information above, infer and create a GitHub issue that describes the underlying problem/feature request that this PR addresses. The issue should be written from the perspective of someone reporting a problem or requesting a feature, NOT describing the solution.

Create a JSON response with these fields:
- "title": A problem-focused title (e.g., "Bug: Authentication fails with expired tokens" or "Feature Request: Add dark mode support")
- "body": Issue body with these sections:
  - **Problem Description**: What issue/need does this PR address?
  - **Current Behavior**: What happens now (for bugs) or what's missing (for features)
  - **Expected Behavior**: What should happen instead
  - **Additional Context**: Any relevant details about impact, use cases, etc.

Focus on the PROBLEM, not the solution. Write as if you're a user reporting an issue or requesting a feature.

Example for a bug fix PR:
{{
  "title": "Bug: User authentication fails when tokens expire",
  "body": "### Problem Description\\n\\nUsers are experiencing authentication failures when their session tokens expire, causing them to lose their work and get logged out unexpectedly.\\n\\n### Current Behavior\\n\\nWhen a token expires, the application throws an error and immediately logs the user out without any warning or graceful handling.\\n\\n### Expected Behavior\\n\\nThe application should detect token expiration and either automatically refresh the token or provide a clear warning to the user before logging them out.\\n\\n### Additional Context\\n\\nThis affects user experience significantly as users lose unsaved work when tokens expire during active sessions."
}}

Respond with only valid JSON.
"""

    def _parse_response(self, response) -> dict:
        """Parse Gemini response to extract issue content."""
        try:
            response_text = response.text.strip()
            if response_text.startswith("```json"):
                response_text = response_text[7:-3]
            elif response_text.startswith("```"):
                response_text = response_text[3:-3]

            issue_data = json.loads(response_text)
            if (
                isinstance(issue_data, dict)
                and "title" in issue_data
                and "body" in issue_data
            ):
                return issue_data
            else:
                print("Invalid issue data structure from Gemini")
                return {}
        except json.JSONDecodeError as exc:
            print(f"Could not parse Gemini response as JSON: {exc}")
            print(f"Response was: {response.text[:200]}...")
            return {}
