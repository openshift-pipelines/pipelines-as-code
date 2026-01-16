"""Analyzes PR content using Gemini AI to suggest labels."""

from __future__ import annotations

import json
from typing import List

import google.generativeai as genai

from .config import DEFAULT_MODEL
from .pr_data import PRData


class GeminiAnalyzer:
    """Analyzes PR content using Gemini AI to suggest labels."""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        genai.configure(api_key=api_key)
        self.model = genai.GenerativeModel(model_name)
        self.model_name = model_name

    def suggest_labels(
        self,
        pr_data: PRData,
        available_labels: List[dict],
        excluded_labels: set[str],
    ) -> List[str]:
        """Analyze PR and suggest appropriate labels."""
        try:
            prompt = self._build_prompt(pr_data, available_labels, excluded_labels)
            response = self.model.generate_content(prompt)
            return self._parse_response(response)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"Error with Gemini API: {exc}")
            return []

    def _build_prompt(
        self,
        pr_data: PRData,
        available_labels: List[dict],
        excluded_labels: set[str],
    ) -> str:
        """Build the prompt for Gemini."""
        commits_text = "\n".join([f"- {msg}" for msg in pr_data.commit_messages])
        files_text = "\n".join(pr_data.files_changed)

        # Format labels with descriptions
        labels_with_descriptions: List[str] = []
        for label in available_labels:
            if label["name"] in excluded_labels:
                continue
            if label["description"]:
                labels_with_descriptions.append(
                    f"{label['name']}: {label['description']}"
                )
            else:
                labels_with_descriptions.append(label["name"])
        labels_text = "\n".join(labels_with_descriptions)

        return f"""
Analyze this GitHub Pull Request and suggest appropriate labels based on the content and intent.

PR Title: {pr_data.title}

PR Description:
{pr_data.description}

Files changed:
{files_text}

Commit messages:
{commits_text}

IMPORTANT: You can ONLY suggest labels from this list of available labels in the repository:
{labels_text}

Based on the PR title, description, files changed, and commit messages, suggest up to 3 relevant labels from the available labels list above. Use the label descriptions to understand their intended purpose.

IMPORTANT RESTRICTIONS:
- Only suggest "documentation" label if files in the docs/ directory are modified
- Only suggest "e2e" label if files in the test/ directory are modified for e2e tests
- Only suggest provider labels ("github", "gitlab", "bitbucket", "gitea") if files in the pkg/provider/ directory are modified
- Provider labels should match the specific provider subdirectory modified (e.g., "github" only if pkg/provider/github/ files are changed)
- Maximum 3 labels total

Respond with only a JSON array of label names that exist in the available labels list, like: ["enhancement", "backend"]
"""

    def _parse_response(self, response) -> List[str]:
        """Parse Gemini response to extract labels."""
        try:
            response_text = response.text.strip()
            if response_text.startswith("```json"):
                response_text = response_text[7:-3]
            elif response_text.startswith("```"):
                response_text = response_text[3:-3]

            labels = json.loads(response_text)
            return labels if isinstance(labels, list) else []
        except json.JSONDecodeError:
            print(f"Could not parse Gemini response as JSON: {response.text}")
            return []


class GeminiReleaseNoteChecker:
    """Checks if a PR requires release notes using Gemini AI."""

    def __init__(self, api_key: str, model_name: str = DEFAULT_MODEL):
        genai.configure(api_key=api_key)
        self.model = genai.GenerativeModel(model_name)
        self.model_name = model_name

    def check_release_note_required(self, pr_data: PRData) -> dict:
        """Analyze PR and determine if release notes are required.

        Returns:
            dict with keys:
            - required: bool - whether release notes are required
            - reason: str - explanation for the decision
        """
        try:
            prompt = self._build_prompt(pr_data)
            response = self.model.generate_content(prompt)
            return self._parse_response(response)
        except Exception as exc:  # pylint: disable=broad-except
            print(f"Error with Gemini API: {exc}")
            # Default to requiring release notes on error
            return {
                "required": True,
                "reason": "Could not analyze PR (defaulting to required)",
            }

    def _build_prompt(self, pr_data: PRData) -> str:
        """Build the prompt for Gemini."""
        commits_text = "\n".join([f"- {msg}" for msg in pr_data.commit_messages])
        files_text = "\n".join(pr_data.files_changed)

        return f"""
You are analyzing a Pull Request for the "Pipelines as Code" project (a Tekton-based CI/CD system) to determine if it requires release notes.

Release notes are ONLY for changes that END USERS of Pipelines as Code need to know about.

PR Title: {pr_data.title}

PR Description:
{pr_data.description}

Files changed:
{files_text}

Commit messages:
{commits_text}

IMPORTANT: Default to NOT requiring release notes unless the change clearly affects end users.

NO RELEASE NOTES NEEDED (return required: false):
- ANY changes to CI/CD pipelines, GitHub Actions, Tekton tasks (.tekton/, .github/workflows/)
- ANY changes to tests (test/, *_test.go, *_test.py)
- ANY changes to development tooling (hack/, scripts/, Makefile, pre-commit)
- ANY changes to linters, formatters, or code quality tools
- Internal refactoring that doesn't change behavior
- Adding/improving logging or error messages (unless user-visible)
- Documentation updates (docs/, README, CONTRIBUTING)
- Dependency updates (go.mod, requirements.txt) unless they fix user-facing bugs
- Code comments or internal string fixes
- Build system changes
- Developer experience improvements
- Flaky test fixes
- Code cleanup or style changes

RELEASE NOTES REQUIRED (return required: true):
- New user-facing features
- Bug fixes that users would notice (not internal/test bugs)
- Changes to CLI commands, flags, or output
- Changes to Kubernetes CRDs or API
- Changes to configuration options users can set
- Security vulnerabilities fixed
- Breaking changes or deprecations
- Changes to webhook handling that affects users
- Changes to how pipelines are triggered or run

Look at the FILES CHANGED carefully:
- If changes are mostly in test/, hack/, .tekton/, .github/ → NO release notes
- If changes are in pkg/ but only add logging/refactor internals → NO release notes  
- If changes affect user-visible behavior in pkg/, cmd/ → MAYBE release notes

Be conservative: when in doubt, return required: false.

Respond with only valid JSON:
{{
  "required": true/false,
  "reason": "Brief explanation"
}}
"""

    def _parse_response(self, response) -> dict:
        """Parse Gemini response."""
        try:
            response_text = response.text.strip()
            if response_text.startswith("```json"):
                response_text = response_text[7:-3]
            elif response_text.startswith("```"):
                response_text = response_text[3:-3]

            result = json.loads(response_text)
            if isinstance(result, dict) and "required" in result:
                return {
                    "required": bool(result.get("required", True)),
                    "reason": result.get("reason", "No reason provided"),
                }
            return {
                "required": True,
                "reason": "Invalid response format (defaulting to required)",
            }
        except json.JSONDecodeError as exc:
            print(f"Could not parse Gemini response as JSON: {exc}")
            return {"required": True, "reason": "Parse error (defaulting to required)"}


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
