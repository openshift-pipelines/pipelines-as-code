"""Main entry point for PR CI utilities."""

import argparse
from typing import List

from .comments import CommentManager
from .config import Config
from .gemini import GeminiAnalyzer, GeminiIssueGenerator
from .github import GitHubClient
from .linter import PRLinter
from .pr_data import PRData
from .utils import check_file_categories, detect_modified_providers


def run_lint() -> None:
    """Run PR linting checks."""
    config = Config.from_env(require_gemini=False)
    if not config:
        print("Error: Missing required environment variables")
        return

    github = GitHubClient(config)
    pr_data = PRData.from_github(github)
    if not pr_data:
        print("Could not fetch PR info for linting")
        return

    linter = PRLinter(pr_data, github)
    linter.check_all()
    linter.report()


def run_update() -> None:
    """Run PR label update using Gemini analysis."""
    config = Config.from_env(require_gemini=True)
    if not config:
        print(
            "Error: Missing required environment variables (including GEMINI_API_KEY)"
        )
        return

    github = GitHubClient(config)
    pr_data = PRData.from_github(github)
    if not pr_data:
        print("Could not fetch PR data")
        return

    print(f"Analyzing PR #{config.pr_number}: {pr_data.title}")
    print(f"Using Gemini model: {config.gemini_model}")
    print(f"Files changed: {len(pr_data.files_changed)} files")
    print(f"Commits: {len(pr_data.commit_messages)} commits")
    print(f"Current labels: {pr_data.current_labels}")

    # Skip if PR already has max_labels or more labels (unless unlimited)
    if config.max_labels > 0 and len(pr_data.current_labels) >= config.max_labels:
        print(
            f"PR already has {len(pr_data.current_labels)} labels "
            f"(max: {config.max_labels}), skipping label addition"
        )
        return

    # Get available labels
    available_labels = github.get_available_labels()
    if not available_labels:
        print("Could not fetch available labels")
        return
    print(f"Available labels in repo: {len(available_labels)} labels")

    # Analyze with Gemini
    analyzer = GeminiAnalyzer(config.gemini_api_key, config.gemini_model)
    suggested_labels = analyzer.suggest_labels(
        pr_data, available_labels, config.excluded_labels
    )
    if not suggested_labels:
        print("No labels suggested by Gemini")
        return

    print(f"Gemini suggested labels: {suggested_labels}")

    # Validate suggested labels exist
    available_label_names = {label["name"] for label in available_labels}
    valid_labels = [
        label for label in suggested_labels if label in available_label_names
    ]
    if len(valid_labels) != len(suggested_labels):
        invalid = [
            label for label in suggested_labels if label not in available_label_names
        ]
        print(f"Warning: Gemini suggested invalid labels: {invalid}")

    # Apply file-based restrictions
    has_docs, has_test, has_provider = check_file_categories(pr_data.files_changed)
    provider_types = (
        detect_modified_providers(pr_data.files_changed) if has_provider else set()
    )

    filtered_labels: List[str] = []
    for label in valid_labels:
        if label == "documentation" and not has_docs:
            print("Skipping 'documentation' label - no docs/ files modified")
            continue
        if label == "e2e" and not has_test:
            print("Skipping 'e2e' label - no test/ files modified")
            continue
        if label in ["github", "gitlab", "bitbucket", "gitea"]:
            if not has_provider:
                print(f"Skipping '{label}' label - no pkg/provider/ files modified")
                continue
            if label not in provider_types:
                print(
                    f"Skipping '{label}' label - no pkg/provider/{label}/ files modified"
                )
                continue
        filtered_labels.append(label)

    # Limit to maximum labels
    if config.max_labels > 0 and len(filtered_labels) > config.max_labels:
        print(f"Limiting labels from {len(filtered_labels)} to {config.max_labels}")
        filtered_labels = filtered_labels[: config.max_labels]

    # Filter out existing labels
    existing_set = set(pr_data.current_labels)
    new_labels = [label for label in filtered_labels if label not in existing_set]

    if new_labels:
        print(f"Adding new labels: {new_labels}")
        github.add_labels(new_labels)
    else:
        print("All suggested labels already exist on the PR")


def run_issue_create() -> None:
    """Generate and create a GitHub issue from PR content."""
    config = Config.from_env(require_gemini=True)
    if not config:
        print(
            "Error: Missing required environment variables (including GEMINI_API_KEY)"
        )
        return

    github = GitHubClient(config)
    pr_data = PRData.from_github(github)
    if not pr_data:
        print("Could not fetch PR data for issue creation")
        return

    print(f"Generating issue for PR #{config.pr_number}: {pr_data.title}")

    # Generate issue content with Gemini
    issue_generator = GeminiIssueGenerator(config.gemini_api_key, config.gemini_model)
    issue_data = issue_generator.generate_issue(pr_data)

    if not issue_data or not issue_data.get("title") or not issue_data.get("body"):
        print("Gemini did not generate valid issue content.")
        return

    print(f"Generated issue: {issue_data['title']}")

    # Create the actual GitHub issue
    created_issue = github.create_issue(
        title=issue_data["title"], body=issue_data["body"]
    )

    if not created_issue:
        print("Failed to create GitHub issue.")
        return

    issue_number = created_issue.get("number")
    issue_url = created_issue.get("html_url")

    # Link the PR to the issue
    link_success = False
    if issue_number:
        link_success = github.link_pr_to_issue(issue_number)
        if link_success:
            print(f"Successfully linked PR to issue #{issue_number}")
        else:
            print(f"Failed to link PR to issue #{issue_number}")

    # Post a comment with the created issue link
    comment_manager = CommentManager(github)
    comment_manager.marker = "<!-- github-issue-created -->"

    status_emoji = "✅" if issue_number else "❌"
    link_status = "and linked to this PR" if link_success else ""

    pretty_comment = f"""{comment_manager.marker}
## {status_emoji} GitHub Issue Created

> **AI-generated issue has been created {link_status}**

### 📋 Created Issue
**[#{issue_number}]({issue_url})** - {issue_data["title"]}

### 🔗 Relationship
This pull request resolves the issue described above. The issue was automatically generated based on the PR content to represent the underlying problem being solved.

### 📝 Issue Content Preview
<details>
<summary>Click to view the generated issue content</summary>

{issue_data["body"]}

</details>

---

<sub>🤖 *Issue created automatically using `/issue-create` command*</sub>"""

    comment_manager.upsert_comment(pretty_comment)
    print(f"Successfully created issue #{issue_number} and posted comment with link.")


def main() -> None:
    """Main entry point."""
    parser = argparse.ArgumentParser(description="PR CI utilities")
    parser.add_argument(
        "command",
        choices=["lint", "update", "all", "issue-create"],
        nargs="?",
        default="all",
        help="Which action to run (default: all)",
    )
    args = parser.parse_args()

    if args.command in ("lint", "all"):
        run_lint()

    if args.command in ("update", "all"):
        run_update()

    if args.command == "issue-create":
        run_issue_create()


if __name__ == "__main__":
    main()
