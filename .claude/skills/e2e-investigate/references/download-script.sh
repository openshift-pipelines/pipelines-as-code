#!/usr/bin/env bash
# E2E Test Artifacts Download Script
# Downloads GitHub Actions artifacts and prepares them for investigation

set -euo pipefail

# Configuration
OUTPUT_DIR="${OUTPUT_DIR:-tmp/e2e}"
REPO="${REPO:-openshift-pipelines/pipelines-as-code}"

# Available patterns for selection
PATTERNS=(
  "flaky"
  "github_1"
  "github_2"
  "gitea_1"
  "gitea_2"
  "gitea_3"
  "github_second_controller"
  "gitlab_bitbucket"
  "concurrency"
)

usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Download E2E test artifacts from GitHub Actions.

Options:
    -p PATTERN    Download artifacts matching pattern (see list below)
    -r RUN_ID     Download from specific run ID
    -u            Extract from zip file in ~/Downloads/
    -l            List recent workflow runs
    -h            Show this help

Available patterns:
$(printf '    %s\n' "${PATTERNS[@]}")

Examples:
    $0 -p github_1           # Download github_1 artifacts
    $0 -l                    # List recent runs
    $0 -u                    # Extract from Downloads zip
    $0                       # Interactive selection with fzf
EOF
}

list_runs() {
  gh run list --repo "$REPO" --workflow "pull-request-e2e" --limit 20
}

download_artifacts() {
  local pattern="$1"
  local timestamp
  timestamp=$(date +%Y%m%d-%H%M%S)
  local dest_dir="${OUTPUT_DIR}/logs-e2e-tests-${pattern}-${timestamp}"

  mkdir -p "$dest_dir"
  echo "Downloading artifacts matching: logs-e2e-tests-${pattern}-*"
  echo "Destination: $dest_dir"

  gh run download --repo "$REPO" \
    --pattern "logs-e2e-tests-${pattern}-*" \
    -D "$dest_dir"

  # Flatten if nested
  for subdir in "$dest_dir"/logs-e2e-tests-"${pattern}"*; do
    if [[ -d "$subdir" ]]; then
      mv "$subdir"/* "$dest_dir/" 2>/dev/null || true
      rmdir "$subdir" 2>/dev/null || true
    fi
  done

  echo "Downloaded to: $dest_dir"
  echo ""
  echo "Files:"
  ls -la "$dest_dir"
}

extract_from_downloads() {
  local zip_file
  zip_file=$(find ~/Downloads -name "logs-e2e-tests-*.zip" -type f -mtime -1 | head -1)

  if [[ -z "$zip_file" ]]; then
    echo "No recent logs-e2e-tests-*.zip found in ~/Downloads/"
    exit 1
  fi

  local basename
  basename=$(basename "$zip_file" .zip)
  local timestamp
  timestamp=$(date +%Y%m%d-%H%M%S)
  local dest_dir="${OUTPUT_DIR}/${basename}-${timestamp}"

  mkdir -p "$dest_dir"
  echo "Extracting: $zip_file"
  echo "Destination: $dest_dir"

  unzip -q "$zip_file" -d "$dest_dir"

  echo "Extracted to: $dest_dir"
  echo ""
  echo "Files:"
  ls -la "$dest_dir"
}

interactive_select() {
  if ! command -v fzf &>/dev/null; then
    echo "fzf not found. Please install fzf or use -p PATTERN"
    exit 1
  fi

  local pattern
  pattern=$(printf '%s\n' "${PATTERNS[@]}" | fzf --prompt="Select pattern: ")

  if [[ -n "$pattern" ]]; then
    download_artifacts "$pattern"
  fi
}

# Parse arguments
while getopts "p:r:ulh" opt; do
  case $opt in
  p)
    download_artifacts "$OPTARG"
    exit 0
    ;;
  r)
    echo "Run ID download not yet implemented"
    exit 1
    ;;
  u)
    extract_from_downloads
    exit 0
    ;;
  l)
    list_runs
    exit 0
    ;;
  h)
    usage
    exit 0
    ;;
  *)
    usage
    exit 1
    ;;
  esac
done

# No arguments - interactive mode
interactive_select
