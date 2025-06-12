#!/usr/bin/env bash
# A script to mirror an external contributor's pull request to a maintainer's fork
# for the purpose of running E2E tests.
#
# Prerequisites:
# 1. GitHub CLI (`gh`) must be installed and authenticated (`gh auth login`).
# 2. You must have a fork of the repository.
# 3. You must have a git remote configured for the upstream repository (e.g., "upstream").
#
# Usage:
# ./mirror-pr.sh <PR_NUMBER> <FORK_REMOTE>
#
# Example:
# ./mirror-pr.sh 1234 my-github-user
#
# if no PR number is provided, it will prompt you to select one using `fzf`.

set -eo pipefail

# --- Prerequisite Check ---

if ! command -v gh &>/dev/null; then
  echo "Error: GitHub CLI ('gh') is not installed. Please install it to continue."
  echo "See: https://cli.github.com/"
  exit 1
fi

echo "✅ GitHub CLI is installed."
# --- Configuration and Argument Parsing ---

PR_NUMBER=$1
FORK_REMOTE=${GH_FORK_REMOTE:-$2}
UPSTREAM_REPO=${GH_UPSTREAM_REPO:-openshift-pipelines/pipelines-as-code}

if [[ -z ${PR_NUMBER} ]]; then
  PR_SELECTION=$(gh pr list --repo "$UPSTREAM_REPO" --json number,title,author --template '{{range .}}{{.number}}: {{.title}} (by {{.author.login}})
{{end}}' | fzf --prompt="Select PR: ")
  PR_NUMBER=$(echo "$PR_SELECTION" | awk -F: '{print $1}' | xargs)
fi

if [[ -z $FORK_REMOTE ]]; then
  FORK_REMOTE=$(git remote | awk '{print $1}' | grep -v origin | sort -u | fzf -1 --prompt="Select fork remote: ")
fi

if [[ -z "$PR_NUMBER" || -z "$FORK_REMOTE" ]]; then
  echo "Usage: $0 <PR_NUMBER> <YOUR_REMOTE_FORK>"
  echo "Example: $0 1234 openshift-pipelines/pipelines-as-code my-github-user"
  exit 1
fi

# --- Main Logic ---

echo "🔄 Fetching details for PR #${PR_NUMBER} from ${UPSTREAM_REPO}..."

# Fetch PR title and author using GitHub CLI
PR_TITLE=$(gh pr view "$PR_NUMBER" --repo "$UPSTREAM_REPO" --json title -q .title)
PR_AUTHOR=$(gh pr view "$PR_NUMBER" --repo "$UPSTREAM_REPO" --json author -q .author.login)
PR_URL="https://github.com/${UPSTREAM_REPO}/pull/${PR_NUMBER}"

if [[ -z "$PR_TITLE" ]]; then
  echo "❌ Error: Could not fetch details for PR #${PR_NUMBER}. Please check the PR number and repository."
  exit 1
fi

echo "  - Title: $PR_TITLE"
echo "  - Author: $PR_AUTHOR"

# 1. Checkout the PR locally
echo "🔄 Checking out PR #${PR_NUMBER} locally..."
gh pr checkout "$PR_NUMBER" --repo "$UPSTREAM_REPO"

# 2. Push the branch to your fork
NEW_BRANCH_NAME="test-pr-${PR_NUMBER}-${PR_AUTHOR}"

echo "🔄 Pushing changes to a new branch '${NEW_BRANCH_NAME}' on your fork (${FORK_REMOTE})..."
# Force push in case the branch already exists from a previous test run
git push "$FORK_REMOTE" "HEAD:${NEW_BRANCH_NAME}" -f

# 3. Create a new Pull Request from the fork to the upstream repo
MIRRORED_PR_TITLE="[MIRRORED] DO NOT MERGE: ${PR_TITLE}"
MIRRORED_PR_BODY="Mirrors ${PR_URL} to run E2E tests. Original author: @${PR_AUTHOR}"
DO_NOT_MERGE_LABEL="do-not-merge" # You might need to create this label in your repo if it doesn't exist

echo "🔄 Creating a new mirrored pull request on ${UPSTREAM_REPO}..."

# Create the PR as a draft to prevent accidental merges before tests run.
# The --head flag specifies the branch in your fork.
CREATED_PR_URL=$(gh pr create \
  --repo "$UPSTREAM_REPO" \
  --title "$MIRRORED_PR_TITLE" \
  --body "$MIRRORED_PR_BODY" \
  --head "${FORK_REMOTE}:${NEW_BRANCH_NAME}" \
  --label "$DO_NOT_MERGE_LABEL" \
  --draft) # Using --draft is safer

# Check if the PR was created successfully
if [[ -z "$CREATED_PR_URL" ]]; then
  echo "❌ Error: Failed to create the mirrored pull request."
  exit 1
fi

gh pr comment "$PR_NUMBER" --repo "$UPSTREAM_REPO" --body "A mirrored PR has been created for E2E testing: ${CREATED_PR_URL}"

echo "✅ Successfully created mirrored pull request!"
echo "   ${CREATED_PR_URL}"

echo "🚀 Done."
