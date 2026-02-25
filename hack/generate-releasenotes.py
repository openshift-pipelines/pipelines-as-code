#!/usr/bin/env python3
# Generate AI-powered release notes for a GitHub release.
#
# Uses the GitHub compare API to find commits between tags, maps them
# to PRs, extracts JIRA tickets (SRVKP-XXXX) from PR bodies and commit
# messages, then sends PR data to Gemini for categorization.  Wraps
# the Gemini output with static header, installation, and changelog
# sections matching the project's established format.
#
# Required env vars:
#   HUB_TOKEN      - GitHub API token
#   GEMINI_API_KEY  - Gemini API key
#   REPO_OWNER      - GitHub org  (e.g. openshift-pipelines)
#   REPO_NAME       - GitHub repo (e.g. pipelines-as-code)
#
# Optional:
#   GEMINI_MODEL    - defaults to gemini-3.1-pro-preview
#
# CLI flags:
#   --current-tag TAG   - override auto-detected current tag (default: tag at HEAD)
#   --stdout            - print release notes to stdout instead of updating the GitHub release
import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request

GITHUB_API = "https://api.github.com"
GEMINI_API = "https://generativelanguage.googleapis.com/v1beta"

# The TODO marker goreleaser puts in the release body.
TODO_RE = re.compile(
    r"TODO: XXXXX.*?see older releases for some example",
    re.DOTALL,
)

JIRA_RE = re.compile(r"(SRVKP-\d+)")


# -- helpers -----------------------------------------------------------------


def env(name, default=None):
    val = os.environ.get(name, default)
    if val is None:
        print(f"error: {name} environment variable is required", file=sys.stderr)
        sys.exit(1)
    return val


def github_get(path, token):
    url = f"{GITHUB_API}{path}" if not path.startswith("https://") else path
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github.v3+json")
    req.add_header("User-Agent", "pac-release-notes")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        print(f"error: GitHub API {exc.code} for {url}", file=sys.stderr)
        raise


def github_patch(path, token, data):
    url = f"{GITHUB_API}{path}"
    body = json.dumps(data).encode()
    req = urllib.request.Request(url, data=body, method="PATCH")
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github.v3+json")
    req.add_header("Content-Type", "application/json")
    req.add_header("User-Agent", "pac-release-notes")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def github_post(path, token, data):
    url = f"{GITHUB_API}{path}"
    body = json.dumps(data).encode()
    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github.v3+json")
    req.add_header("Content-Type", "application/json")
    req.add_header("User-Agent", "pac-release-notes")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def git(*args):
    result = subprocess.run(["git", *args], capture_output=True, text=True, check=True)
    return result.stdout.strip()


# -- main logic --------------------------------------------------------------


def get_current_tag():
    """Return the tag pointing at HEAD, or None."""
    out = git("tag", "--points-at", "HEAD")
    tags = [t for t in out.splitlines() if t.startswith("v")]
    return tags[0] if tags else None


def get_previous_local_tag(current_tag):
    """Return the previous version tag using local git's version sort."""
    out = git("tag", "--list", "v*", "--sort=-version:refname")
    tags = [t for t in out.splitlines() if t]
    try:
        idx = tags.index(current_tag)
    except ValueError:
        return None
    if idx + 1 >= len(tags):
        return None
    return tags[idx + 1]


def get_previous_tag_from_github(current_tag, owner, repo, token):
    """Return the tag immediately before *current_tag* using the GitHub tags API."""
    page = 1
    found = False
    while True:
        tags = github_get(f"/repos/{owner}/{repo}/tags?per_page=100&page={page}", token)
        if not tags:
            break
        for t in tags:
            name = t["name"]
            if not name.startswith("v"):
                continue
            if name == current_tag:
                found = True
                continue
            if found:
                return name
        page += 1
    return None


def github_tag_exists(owner, repo, tag, token):
    """Return True if *tag* exists on GitHub."""
    try:
        github_get(f"/repos/{owner}/{repo}/git/ref/tags/{tag}", token)
        return True
    except urllib.error.HTTPError as exc:
        if exc.code == 404:
            return False
        raise


def get_tag_commit_sha(tag):
    """Return the commit SHA for a local tag (handles annotated tags)."""
    return git("rev-list", "-n", "1", "--", tag)


def _commits_from_compare(owner, repo, old_tag, new_tag, token):
    """Try the GitHub compare API.  Returns a list of commit objects or None on failure."""
    path = f"/repos/{owner}/{repo}/compare/{old_tag}...{new_tag}"
    try:
        data = github_get(path, token)
    except urllib.error.HTTPError:
        return None
    commits = data.get("commits", [])
    total = data.get("total_commits", len(commits))
    if total > len(commits):
        print(
            f"warning: compare returned {len(commits)} of {total} commits, "
            "fetching remaining via commit list API...",
            file=sys.stderr,
        )
        page = 1
        all_shas = {c["sha"] for c in commits}
        seen_page_signatures = set()
        while len(commits) < total:
            try:
                extra = github_get(
                    f"/repos/{owner}/{repo}/commits?sha={new_tag}&per_page=100&page={page}",
                    token,
                )
            except urllib.error.HTTPError:
                break
            if not extra:
                break
            page_signature = tuple(c["sha"] for c in extra)
            if page_signature in seen_page_signatures:
                print(
                    f"warning: commit list pagination repeated page {page}, "
                    "stopping to avoid infinite loop",
                    file=sys.stderr,
                )
                break
            seen_page_signatures.add(page_signature)
            for c in extra:
                if c["sha"] in all_shas:
                    continue
                all_shas.add(c["sha"])
                commits.append(c)
            page += 1
    return commits


def fetch_compare_data(owner, repo, old_ref, new_ref, token):
    """Get commits between refs and map them to PRs with JIRA tickets.

    Returns a deduplicated list of PR/commit dicts with jira_tickets.
    """
    commits = _commits_from_compare(owner, repo, old_ref, new_ref, token)
    if commits is None:
        print("error: compare API failed", file=sys.stderr)
        sys.exit(1)

    seen_prs = set()
    pr_entries = []

    for c in commits:
        sha = c["sha"]
        commit_msg = c["commit"]["message"]
        subject = commit_msg.split("\n", 1)[0]
        author_login = (c.get("author") or {}).get("login", "")
        if not author_login:
            author_login = c["commit"]["author"]["name"]

        # Look up associated PR
        try:
            pr_data = github_get(f"/repos/{owner}/{repo}/commits/{sha}/pulls", token)
        except urllib.error.HTTPError:
            pr_data = []

        if pr_data:
            for pr in pr_data:
                pr_number = pr["number"]
                if pr_number in seen_prs:
                    continue
                seen_prs.add(pr_number)
                pr_body = pr.get("body") or ""
                pr_title = pr.get("title") or ""
                jira_tickets = sorted(
                    set(
                        JIRA_RE.findall(pr_title)
                        + JIRA_RE.findall(pr_body)
                        + JIRA_RE.findall(commit_msg)
                    )
                )
                pr_entries.append(
                    {
                        "kind": "pr",
                        "number": pr_number,
                        "title": pr_title,
                        "body": pr_body[:500],
                        "labels": [label["name"] for label in pr.get("labels", [])],
                        "author": (pr.get("user") or {}).get("login", "unknown"),
                        "url": pr["html_url"],
                        "jira_tickets": jira_tickets,
                    }
                )
        else:
            jira_tickets = sorted(set(JIRA_RE.findall(commit_msg)))
            pr_entries.append(
                {
                    "kind": "commit",
                    "sha": sha[:7],
                    "title": subject,
                    "body": commit_msg[:500],
                    "labels": [],
                    "author": author_login,
                    "url": f"https://github.com/{owner}/{repo}/commit/{sha}",
                    "jira_tickets": jira_tickets,
                }
            )

    return pr_entries


def build_prompt(prs, current_tag, previous_tag):
    items = []
    for p in prs:
        if p["kind"] == "pr":
            header = f"- PR #{p['number']} by @{p['author']}: {p['title']}"
        else:
            header = f"- Commit {p['sha']} by {p['author']}: {p['title']}"
        jira_str = ", ".join(p.get("jira_tickets", []))
        items.append(
            f"{header}\n"
            f"  Labels: {', '.join(p['labels']) or 'none'}\n"
            f"  URL: {p['url']}\n"
            f"  JIRA tickets: {jira_str or 'none'}\n"
            f"  Description: {p['body'][:300]}"
        )
    pr_text = "\n".join(items)
    return f"""\
You are writing release notes for the open-source project "Pipelines as Code".
This release is {current_tag} (previous release was {previous_tag}).

Below is the list of pull requests and commits in this release:

{pr_text}

Write categorized release notes in GitHub-flavoured Markdown using EXACTLY these sections (skip empty ones):

## ✨ Major changes and Features
## 🐛 Bug Fixes
## 📚 Documentation Updates
## ⚙️ Chores

For each entry, use EXACTLY this format.  The Link and Jira lines MUST be \
indented with two spaces so they render as nested sub-bullets:

* **Bold title:** One-sentence description of the change.
  * Link: <PR_OR_COMMIT_URL>
  * Jira: [SRVKP-XXXX](https://issues.redhat.com/browse/SRVKP-XXXX)

Rules:
- The first bullet MUST start with "* " (no indent) with a bold title followed by a colon and a description.
- The Link line MUST start with "  * Link:" (two-space indent) with the PR or commit URL.
- The Jira line MUST start with "  * Jira:" (two-space indent). Include it ONLY if the entry \
has JIRA tickets listed above. Use the ticket IDs provided. If there are multiple tickets, \
list each as a separate markdown link comma-separated.
- Within each section, list entries that have JIRA tickets FIRST, before entries without JIRA tickets.
- Do NOT add a Contributors section.
- Do NOT add any header or footer outside the sections above.
- Output ONLY the Markdown sections, no extra commentary."""


def build_header(tag):
    return f"""\
# Pipelines as Code version {tag}

OpenShift Pipelines as Code {tag} has been released 🥳"""


def build_installation(tag, owner, repo):
    tag_dashed = tag.replace(".", "-")
    return f"""\
## Installation

To install this version you can install the release.yaml with [`kubectl`](https://kubernetes.io/docs/tasks/tools/) for your platform :

### Openshift

```shell
kubectl apply -f https://github.com/{owner}/{repo}/releases/download/{tag}/release.yaml
```

### Kubernetes

```shell
kubectl apply -f https://github.com/{owner}/{repo}/releases/download/{tag}/release.k8s.yaml
```

### Documentation

The documentation for this release is available here :

https://release-{tag_dashed}.pipelines-as-code.pages.dev"""


def fetch_github_changelog(
    owner, repo, current_tag, previous_tag, token, target_commitish=None
):
    """Fetch auto-generated release notes from GitHub."""
    payload = {"tag_name": current_tag, "previous_tag_name": previous_tag}
    if target_commitish:
        payload["target_commitish"] = target_commitish
    data = github_post(
        f"/repos/{owner}/{repo}/releases/generate-notes",
        token,
        payload,
    )
    return data["body"]


def call_gemini(prompt, api_key, model):
    url = f"{GEMINI_API}/models/{model}:generateContent"
    payload = {
        "contents": [{"parts": [{"text": prompt}]}],
    }
    body = json.dumps(payload).encode()
    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("x-goog-api-key", api_key)
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())
    try:
        return data["candidates"][0]["content"]["parts"][0]["text"]
    except (KeyError, IndexError):
        print(
            f"error: unexpected Gemini response: {json.dumps(data)}",
            file=sys.stderr,
        )
        sys.exit(1)


def update_release_body(owner, repo, tag, notes, token):
    """Replace the TODO placeholder in the GitHub release body."""
    release = github_get(f"/repos/{owner}/{repo}/releases/tags/{tag}", token)
    old_body = release.get("body", "")

    if not TODO_RE.search(old_body):
        print("warning: TODO placeholder not found in release body, appending notes")
        new_body = notes + "\n\n" + old_body
    else:
        new_body = TODO_RE.sub(notes, old_body)

    github_patch(
        f"/repos/{owner}/{repo}/releases/{release['id']}",
        token,
        {"body": new_body},
    )
    print(f"Release {tag} updated successfully")


def parse_args():
    parser = argparse.ArgumentParser(description="Generate AI-powered release notes")
    parser.add_argument(
        "--current-tag",
        help="override auto-detected current tag (default: tag at HEAD)",
    )
    parser.add_argument(
        "--stdout",
        action="store_true",
        help="print notes to stdout instead of updating the GitHub release",
    )
    parser.add_argument(
        "--model",
        help="Gemini model to use (default: GEMINI_MODEL env or gemini-3.1-pro-preview)",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    token = env("HUB_TOKEN")
    api_key = env("GEMINI_API_KEY")
    owner = env("REPO_OWNER")
    repo = env("REPO_NAME")
    model = args.model or os.environ.get("GEMINI_MODEL", "gemini-3.1-pro-preview")

    current_tag = args.current_tag or get_current_tag()
    if not current_tag:
        print("No tag at HEAD, skipping release notes generation")
        return

    previous_tag = get_previous_local_tag(current_tag)
    if not previous_tag:
        previous_tag = get_previous_tag_from_github(current_tag, owner, repo, token)
    if not previous_tag:
        print(
            f"No previous tag found before {current_tag} (local or GitHub), skipping",
        )
        return

    compare_new_ref = current_tag
    target_commitish = None
    if not github_tag_exists(owner, repo, current_tag, token):
        if not args.stdout:
            print(
                f"Tag {current_tag} does not exist on GitHub yet. "
                "Push the tag first or use --stdout for a local preview.",
                file=sys.stderr,
            )
            sys.exit(1)
        target_commitish = get_tag_commit_sha(current_tag)
        compare_new_ref = target_commitish
        print(
            f"warning: {current_tag} is not on GitHub; using commit "
            f"{target_commitish[:12]} for compare/generate-notes preview",
            file=sys.stderr,
        )

    print(f"Generating release notes for {current_tag} (previous: {previous_tag})")

    print("Fetching compare data from GitHub...")
    pr_entries = fetch_compare_data(owner, repo, previous_tag, compare_new_ref, token)
    if not pr_entries:
        print("No commits between tags, skipping")
        return

    print(
        f"Found {len(pr_entries)} PRs/commits, calling Gemini ({model})...",
    )
    prompt = build_prompt(pr_entries, current_tag, previous_tag)
    gemini_notes = call_gemini(prompt, api_key, model)

    header = build_header(current_tag)
    installation = build_installation(current_tag, owner, repo)
    changelog = fetch_github_changelog(
        owner,
        repo,
        current_tag,
        previous_tag,
        token,
        target_commitish=target_commitish,
    )
    notes = f"{header}\n\n{gemini_notes}\n\n{installation}\n\n{changelog}\n"

    if args.stdout:
        print(notes)
    else:
        print("Updating GitHub release...")
        update_release_body(owner, repo, current_tag, notes, token)


if __name__ == "__main__":
    main()
