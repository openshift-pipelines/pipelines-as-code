"""Data about a pull request."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional

from .github import GitHubClient


@dataclass
class PRData:
    """Data about a pull request."""

    title: str
    description: str
    files_changed: List[str]
    commit_messages: List[str]
    pr_info: Optional[dict]
    current_labels: List[str] = field(default_factory=list)

    @classmethod
    def from_github(cls, github: GitHubClient) -> Optional["PRData"]:
        """Fetch PR data from GitHub API."""
        pr_info = github.get_pr_info()
        if not pr_info:
            return None

        title = pr_info.get("title", "")
        description = pr_info.get("body", "") or ""
        files_changed = github.get_pr_files()

        commits_data = github.get_pr_commits()
        commit_messages = [
            commit.get("commit", {}).get("message", "")
            for commit in commits_data
            if commit.get("commit", {}).get("message", "")
        ]

        current_labels = [label["name"] for label in pr_info.get("labels", [])]

        return cls(
            title=title,
            description=description,
            files_changed=files_changed,
            commit_messages=commit_messages,
            pr_info=pr_info,
            current_labels=current_labels,
        )
