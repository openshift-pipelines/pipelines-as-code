#!/usr/bin/env bash
#
no_verify=
show_help() {
  cat <<EOF
🪞 Mirror an external contributor's pull request to a maintainer's fork for E2E tests.

🛠️ Prerequisites:
1. 🐙 GitHub CLI (gh) must be installed and authenticated (gh auth login).
2. 🍴 You must have a fork of the repository.
3. 🔗 You must have a git remote configured for the upstream repository (e.g., "upstream").
4. 👾 You need fzf and jq installed for selecting PRs and parsing JSON.

▶️ Usage:
  ./mirror-pr.sh <PR_NUMBER> <FORK_REMOTE>

💡 Example:
  ./mirror-pr.sh 1234 my-github-user

If no PR number or not fork are provided, it will prompt you to select one
using fzf.

EOF
  grep -E "[ ]*[a-zA-Z0-9-]\) ##" $0 |
    sed -e 's/^[ ]*/-/' \
      -e 's/-\([0-9A-Za-z]*\)[  ]*|[  ]*\([0-9A-Za-z]*\)/-\1, -\2/' \
      -e 's/##//' -e 's/)[ ]*/ - /' |
    awk -F" - " '{printf "%-10s %s\n", $1, $2}'

  cat <<EOF

EOF
}
while getopts "hn" opt; do
  case $opt in
  n) ## do not run pre-commit checks
    no_verify=yes
    ;;
  h)
    echo "usage: $(basename $(readlink -f $0))"
    show_help
    exit 0
    ;;
  *)
    echo "unknown option: -${OPTARG}" >&2
    show_help
    exit 1
    ;;
  esac
done
shift $((OPTIND - 1))

set -eo pipefail

if ! command -v gh &>/dev/null; then
  echo "🛑 Error: GitHub CLI ('gh') is not installed. Please install it to continue."
  echo "🔗 See: https://cli.github.com/"
  exit 1
fi

echo "✅ GitHub CLI is installed. Ready to proceed!"

PR_NUMBER=${1:-}
FORK_REMOTE=${GH_FORK_REMOTE:-$2}
UPSTREAM_REPO=${GH_UPSTREAM_REPO:-"openshift-pipelines/pipelines-as-code"}

# 🛡️ Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
  echo "📝 Error: There are uncommitted changes in the current branch. Please commit or stash them before running this script."
  exit 1
fi

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
resetgitbranch() {
  new_branch_name=$(git rev-parse --abbrev-ref HEAD)
  echo "↩️  Resetting to original branch ${CURRENT_BRANCH} from ${new_branch_name}"
  git checkout "$CURRENT_BRANCH" || true
}
trap resetgitbranch EXIT

# 🎯 Select PR number if not provided
if [[ -z ${PR_NUMBER} ]]; then
  if [[ ${CURRENT_BRANCH} =~ test-pr-([0-9]+)-([a-zA-Z0-9_-]+) ]]; then
    PR_NUMBER="${BASH_REMATCH[1]}"
  else
    PR_SELECTION=$(gh pr list --repo "$UPSTREAM_REPO" --json number,title,author --template '{{range .}}{{.number}}: {{.title}} (by {{.author.login}})
{{end}}' | grep -v "\[MIRRORED\]" | fzf --prompt="🔎 Select PR: ")
    PR_NUMBER=$(echo "$PR_SELECTION" | awk -F: '{print $1}' | xargs)
  fi
fi

# 🔍 Check if a mirrored PR already exists
already_opened_pr=$(
  gh pr list --repo $UPSTREAM_REPO \
    --json number,headRepositoryOwner,headRepository,headRefName |
    jq -r --arg pn "$PR_NUMBER" \
      '.[] | select(.headRefName | test("^test-pr-\($pn)-.*")) | "git@github.com:\(.headRepositoryOwner.login)/\(.headRepository.name).git"'
)
[[ -n ${already_opened_pr} ]] && FORK_REMOTE=${already_opened_pr}

# 🌿 Select fork remote if not provided
if [[ -z $FORK_REMOTE ]]; then
  FORK_REMOTE=$(git remote | awk '{print $1}' | grep -v origin | sort -u | fzf -1 --prompt="🌿 Select fork remote: ")
fi

if [[ -z "$PR_NUMBER" || -z "$FORK_REMOTE" ]]; then
  echo "ℹ️  Usage: $0 <PR_NUMBER> <YOUR_REMOTE_FORK>"
  echo "💡 Example: $0 1234 my-github-user"
  echo "🔎 UPSTREAM_REPO is ${UPSTREAM_REPO} unless you configure the env variable GH_UPSTREAM_REPO."
  exit 1
fi

echo "🔍 Fetching details for PR #${PR_NUMBER} from ${UPSTREAM_REPO}..."
echo "🌿 Fork remote: ${FORK_REMOTE}"

# 📋 Fetch PR title and author
PR_TITLE=$(gh pr view "$PR_NUMBER" --repo "$UPSTREAM_REPO" --json title -q .title)
PR_AUTHOR=$(gh pr view "$PR_NUMBER" --repo "$UPSTREAM_REPO" --json author -q .author.login)
PR_URL="https://github.com/${UPSTREAM_REPO}/pull/${PR_NUMBER}"

if [[ -z "$PR_TITLE" ]]; then
  echo "❌ Error: Could not fetch details for PR #${PR_NUMBER}. Please check the PR number and repository."
  exit 1
fi

echo "📝  - Title: $PR_TITLE"
echo "👤  - Author: $PR_AUTHOR"

# 1️⃣ Checkout the PR locally
echo "📥 Checking out PR #${PR_NUMBER} locally..."
gh pr checkout --force "$PR_NUMBER" --repo "$UPSTREAM_REPO"

# 2️⃣ Push the branch to your fork
NEW_BRANCH_NAME="test-pr-${PR_NUMBER}-${PR_AUTHOR}"

if [[ -n ${already_opened_pr} ]]; then
  echo "🔁 A pull request already exists for this branch, pushing to the pull request target: ${already_opened_pr}"
  echo "🚚 Pushing changes to existing pull request branch '${NEW_BRANCH_NAME}' fork (${FORK_REMOTE})..."
else
  echo "🚀 Pushing changes to a new branch '${NEW_BRANCH_NAME}' on your fork (${FORK_REMOTE})..."
fi

# 🚨 Force push in case the branch already exists from a previous test run
if [[ -n ${no_verify} ]]; then
  echo "⚠️  Skipping pre-push verification due to --no-verify flag."
else
  if ! command -v "pre-commit" >/dev/null 2>&1; then
    echo "⚠️ You need to have the 'pre-commit' tool installed to run this script."
    exit 1
  fi
  echo "🚜 Running pre-commit checks before pushing..."
  pre-commit run --all-files --show-diff-on-failure || {
    echo "❗ Pre-commit checks failed. Please fix the issues before pushing."
    echo "You can fix user errors locally and pushing to the user branch."
    echo "git commit --amend the commit (or add a new commit) and then run this command"
    gh pr view "$PR_NUMBER" --repo "$UPSTREAM_REPO" --json headRefName,headRepositoryOwner,headRepository |
      jq -r '"git push --force-with-lease git@github.com:\(.headRepositoryOwner.login)/\(.headRepository.name).git HEAD:\(.headRefName)"'
    echo "(or use --force if you know what you are doing)"
    exit 1
  }
fi
git push "$FORK_REMOTE" "HEAD:${NEW_BRANCH_NAME}" --force --no-verify

if [[ -n ${already_opened_pr} ]]; then
  exit 0
fi

# 3️⃣ Create a new Pull Request from the fork to the upstream repo
MIRRORED_PR_TITLE="🪞 [MIRRORED] ${PR_TITLE}"
MIRRORED_PR_BODY="🔄 Mirrors ${PR_URL} to run E2E tests. Original author: @${PR_AUTHOR}"
DO_NOT_MERGE_LABEL="do-not-merge" # You might need to create this label in your repo if it doesn't exist

echo "🆕 Creating a new mirrored pull request on ${UPSTREAM_REPO}..."

# 📝 Create the PR as a draft to prevent accidental merges before tests run.
CREATED_PR_URL=$(gh pr create \
  --repo "$UPSTREAM_REPO" \
  --title "$MIRRORED_PR_TITLE" \
  --body "$MIRRORED_PR_BODY" \
  --head "${FORK_REMOTE}:${NEW_BRANCH_NAME}" \
  --label "$DO_NOT_MERGE_LABEL" \
  --draft)

# ✅ Check if the PR was created successfully
if [[ -z "$CREATED_PR_URL" ]]; then
  echo "❗ Error: Failed to create the mirrored pull request."
  exit 1
fi

gh pr comment "$PR_NUMBER" --repo "$UPSTREAM_REPO" --body \
  "🚀 **Mirrored PR Created for E2E Testing**<br><br>\
A mirrored PR has been opened for end-to-end testing: [View PR](${CREATED_PR_URL})<br><br>\
⏳ Follow progress there for E2E results.<br>\
If you need to update the PR with new changes, please ask a maintainer to rerun \`hack/mirror-pr.sh\`."

echo "🎉 Successfully created mirrored pull request!"
echo "   ${CREATED_PR_URL}"

echo "🏁 Done."
