#!/usr/bin/env -S uv --quiet run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "requests",
#   "google-generativeai",
# ]
# ///
import json
import os
from collections import Counter

# pylint: disable=no-name-in-module
import google.generativeai as genai
import requests

DEFAULT_MODEL = "gemini-2.5-flash"


def get_paginated_data(url, headers, timeout=300):
    """Helper function to fetch all pages from a GitHub API endpoint"""
    all_data = []
    while url:
        response = requests.get(url, headers=headers, timeout=timeout)
        response.raise_for_status()
        data = response.json()
        all_data.extend(data)

        # Get next page URL from Link header
        link_header = response.headers.get("Link", "")
        url = None
        for link in link_header.split(","):
            if 'rel="next"' in link:
                url = link.split(";")[0].strip("<> ")
                break

    return all_data


def get_excluded_labels():
    """Get excluded labels from environment variable or use default"""
    excluded_env = os.environ.get(
        "EXCLUDED_LABELS", "good-first-issue,help-wanted,wontfix"
    )
    # Split by comma and strip whitespace, filter out empty strings
    return {label.strip() for label in excluded_env.split(",") if label.strip()}


def get_pr_data():
    """Get PR description, files changed, all commit messages, and PR info from GitHub API"""
    try:
        # Get PR info
        pr_info = get_current_pr_info()
        if not pr_info:
            return "", "", [], [], None

        pr_description = pr_info.get("body", "") or ""
        pr_title = pr_info.get("title", "")

        # Get files changed
        files_url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/pulls/{os.environ['PR_NUMBER']}/files"
        headers = {
            "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
            "Accept": "application/vnd.github.v3+json",
        }

        files_data = get_paginated_data(files_url, headers)

        files_changed = []
        for file_info in files_data:
            status = file_info.get("status", "modified")[0].upper()  # M, A, D, etc.
            filename = file_info.get("filename", "")
            files_changed.append(f"{status}\t{filename}")

        # Get all commits in the PR
        commits_url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/pulls/{os.environ['PR_NUMBER']}/commits"
        commits_data = get_paginated_data(commits_url, headers)

        commit_messages = []
        for commit in commits_data:
            message = commit.get("commit", {}).get("message", "")
            if message:
                commit_messages.append(message)

        return pr_title, pr_description, files_changed, commit_messages, pr_info

    except requests.exceptions.RequestException as e:
        print(f"Error fetching PR data: {e}")
        return "", "", [], [], None


def analyze_with_gemini(
    pr_title, pr_description, files_changed, commit_messages, available_labels
):
    try:
        genai.configure(api_key=os.environ["GEMINI_API_KEY"])
        model_name = os.environ.get("GEMINI_MODEL", DEFAULT_MODEL)
        model = genai.GenerativeModel(model_name)

        # Format commit messages
        commits_text = "\n".join([f"- {msg}" for msg in commit_messages])
        files_text = "\n".join(files_changed)

        # Format labels with descriptions (exclude certain labels)
        excluded_labels = get_excluded_labels()
        labels_with_descriptions = []
        for label in available_labels:
            if label["name"] in excluded_labels:
                continue  # Skip excluded labels
            if label["description"]:
                labels_with_descriptions.append(
                    f"{label['name']}: {label['description']}"
                )
            else:
                labels_with_descriptions.append(label["name"])
        labels_text = "\n".join(labels_with_descriptions)

        prompt = f"""
Analyze this GitHub Pull Request and suggest appropriate labels based on the content and intent.

PR Title: {pr_title}

PR Description:
{pr_description}

Files changed:
{files_text}

Commit messages:
{commits_text}

IMPORTANT: You can ONLY suggest labels from this list of available labels in the repository:
{labels_text}

Based on the PR title, description, files changed, and commit messages, suggest 2-4 relevant labels from the available labels list above. Use the label descriptions to understand their intended purpose.

Respond with only a JSON array of label names that exist in the available labels list, like: ["enhancement", "backend"]
"""

        response = model.generate_content(prompt)

        try:
            # Extract JSON from response
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

    except Exception as e:
        print(f"Error with Gemini API: {e}")
        return []


def get_available_labels():
    """Get all available labels in the repository with their descriptions"""
    url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/labels"
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }

    try:
        labels_data = get_paginated_data(url, headers)

        available_labels = []
        for label in labels_data:
            label_info = {
                "name": label["name"],
                "description": label.get("description", "") or "",
            }
            available_labels.append(label_info)

        return available_labels
    except requests.exceptions.RequestException as e:
        print(f"Error fetching available labels: {e}")
        return []


def get_current_pr_info():
    url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/pulls/{os.environ['PR_NUMBER']}"
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }

    try:
        response = requests.get(url, headers=headers, timeout=300)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"Error fetching PR info: {e}")
        return None


def add_labels_to_pr(labels):
    if not labels:
        print("No labels to add")
        return

    url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/issues/{os.environ['PR_NUMBER']}/labels"
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }

    try:
        response = requests.post(url, headers=headers, json=labels, timeout=300)
        response.raise_for_status()
        print(f"Successfully added labels: {labels}")
    except requests.exceptions.RequestException as e:
        print(f"Error adding labels: {e}")
        if hasattr(e, "response") and e.response:
            print(f"Response: {e.response.text}")


def add_reviewers_to_pr(reviewers):
    """Add reviewers to the current pull request"""
    if not reviewers:
        print("No reviewers to add")
        return

    url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/pulls/{os.environ['PR_NUMBER']}/requested_reviewers"
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }
    data = {"reviewers": reviewers}

    try:
        response = requests.post(url, headers=headers, json=data, timeout=300)
        response.raise_for_status()
        print(f"Successfully added reviewers: {reviewers}")
    except requests.exceptions.RequestException as e:
        print(f"Error adding reviewers: {e}")
        if hasattr(e, "response") and e.response:
            print(f"Response: {e.response.text}")


def handle_reviewer_suggestions(pr_info, files_changed):
    """Suggests and adds reviewers based on file change history if none are assigned"""
    if pr_info.get("requested_reviewers"):
        assigned_reviewers = [r["login"] for r in pr_info["requested_reviewers"]]
        print(f"PR already has reviewers assigned: {assigned_reviewers}")
        return

    print("No reviewers assigned, attempting to suggest one...")
    pr_author = pr_info.get("user", {}).get("login")
    headers = {
        "Authorization": f"token {os.environ['GITHUB_TOKEN']}",
        "Accept": "application/vnd.github.v3+json",
    }

    # Filter out vendor files
    non_vendor_files = [
        f for f in files_changed if not f.split("\t")[1].startswith("vendor/")
    ]

    # Limit to the first 50 files
    files_to_process = non_vendor_files
    if len(non_vendor_files) > 50:
        print(
            "PR has more than 50 non-vendor files, analyzing the first 50 for reviewer suggestion."
        )
        files_to_process = non_vendor_files[:50]

    contributor_counts = Counter()
    for file_info in files_to_process:
        filename = file_info.split("\t")[1]
        # Limit to 20 commits per file
        commits_url = f"https://api.github.com/repos/{os.environ['REPO_OWNER']}/{os.environ['REPO_NAME']}/commits?path={filename}&per_page=20"
        try:
            # Use a direct request instead of pagination for only the first page
            response = requests.get(commits_url, headers=headers, timeout=300)
            response.raise_for_status()
            commits_data = response.json()
            for commit in commits_data:
                if commit.get("author") and commit["author"].get("login"):
                    author = commit["author"]["login"]
                    if author != pr_author:
                        contributor_counts[author] += 1
        except requests.exceptions.RequestException as e:
            print(f"Could not fetch commits for {filename}: {e}")
            continue
    if not contributor_counts:
        print("Could not identify any potential reviewers from file history.")
    else:
        top_reviewer = contributor_counts.most_common(1)[0][0]
        print(f"Suggested reviewer based on file history: {top_reviewer}")
        add_reviewers_to_pr([top_reviewer])


def validate_environment():
    """Validate all required environment variables are set"""
    required_vars = [
        "GITHUB_TOKEN",
        "REPO_OWNER",
        "REPO_NAME",
        "PR_NUMBER",
        "GEMINI_API_KEY",
    ]
    missing_vars = []

    for var in required_vars:
        if not os.environ.get(var):
            missing_vars.append(var)

    if missing_vars:
        print(
            f"Error: Missing required environment variables: {', '.join(missing_vars)}"
        )
        return False

    return True


def main():
    # Validate environment variables first
    if not validate_environment():
        return

    # Get PR data from GitHub API
    pr_title, pr_description, files_changed, commit_messages, pr_info = get_pr_data()
    if not pr_title:
        print("Could not fetch PR data")
        return

    print(f"Analyzing PR #{os.environ['PR_NUMBER']}: {pr_title}")

    # --- Suggest and add reviewers ---
    handle_reviewer_suggestions(pr_info, files_changed)

    # --- ORIGINAL LABELING LOGIC (UNCHANGED) ---
    # Show which Gemini model is being used
    model_name = os.environ.get("GEMINI_MODEL", DEFAULT_MODEL)
    print(f"Using Gemini model: {model_name}")

    # Get available labels in the repository
    available_labels = get_available_labels()
    if not available_labels:
        print("Could not fetch available labels")
        return

    print(f"Available labels in repo: {len(available_labels)} labels")

    # Get current labels from the PR info we already fetched
    current_labels = []
    if pr_info:
        current_labels = [label["name"] for label in pr_info.get("labels", [])]

    print(f"Current labels: {current_labels}")
    print(f"Files changed: {len(files_changed)} files")
    print(f"Commits: {len(commit_messages)} commits")

    # Analyze with Gemini
    suggested_labels = analyze_with_gemini(
        pr_title, pr_description, files_changed, commit_messages, available_labels
    )
    if not suggested_labels:
        print("No labels suggested by Gemini")
        return

    print(f"Gemini suggested labels: {suggested_labels}")

    # Create sets for filtering - extract label names from the new structure
    available_label_names = {label["name"] for label in available_labels}
    existing_labels_set = set(current_labels)

    # Ensure suggested labels exist in repo
    valid_suggested_labels = [
        label for label in suggested_labels if label in available_label_names
    ]
    if len(valid_suggested_labels) != len(suggested_labels):
        invalid_labels = [
            label for label in suggested_labels if label not in available_label_names
        ]
        print(f"Warning: Gemini suggested invalid labels: {invalid_labels}")

    # Filter out labels that already exist
    new_labels = [
        label for label in valid_suggested_labels if label not in existing_labels_set
    ]

    if new_labels:
        print(f"Adding new labels: {new_labels}")
        add_labels_to_pr(new_labels)
    else:
        print("All suggested labels already exist on the PR")


if __name__ == "__main__":
    main()
