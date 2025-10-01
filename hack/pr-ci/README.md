# PR CI Utilities

A Python package that provides automated PR (Pull Request) analysis and enhancement capabilities using AI. This tool is designed to work within Tekton Pipelines as Code workflows to automatically lint, label, and generate issues from pull requests.

## Features

### 1. PR Linting (`lint`)

- **Conventional Commits**: Validates PR titles against conventional commit format
- **Description Completeness**: Ensures PR descriptions have sufficient detail
- **Template Usage**: Checks if PR description was customized from the default template
- **Jira/GitHub References**: Verifies presence of Jira tickets or GitHub issue references
- **AI Attribution**: Validates that commits using AI tools include proper attribution footers

### 2. Automated Labeling (`update`)

- **AI-Powered Analysis**: Uses Google Gemini to analyze PR content and suggest relevant labels
- **Smart Filtering**: Applies file-based restrictions (e.g., "documentation" label only for docs/ changes)
- **Provider Detection**: Automatically detects changes to specific providers (github, gitlab, etc.)
- **Label Validation**: Ensures suggested labels exist in the repository

### 3. Issue Generation (`issue-create`)

- **AI-Generated Issues**: Creates structured GitHub issues from PR content using Gemini
- **Comprehensive Analysis**: Analyzes PR title, description, file changes, and commit messages
- **Structured Output**: Generates issues with Summary, Changes, and Testing sections
- **Comment Integration**: Posts generated content as PR comments for review

## Usage

### Command Line Interface

```bash
# Run all checks (lint + update)
uv run pr-ci all

# Run only linting checks
uv run pr-ci lint

# Run only label updates
uv run pr-ci update

# Generate GitHub issue content
uv run pr-ci issue-create
```

### Environment Variables

The following environment variables are required:

#### Required for all operations

- `GITHUB_TOKEN`: GitHub API token with repo access
- `REPO_OWNER`: Repository owner (e.g., "openshift-pipelines")
- `REPO_NAME`: Repository name (e.g., "pipelines-as-code")
- `PR_NUMBER`: Pull request number

#### Required for AI operations (`update`, `issue-create`)

- `GEMINI_API_KEY`: Google Gemini API key

#### Optional

- `GEMINI_MODEL`: Gemini model to use (default: "gemini-2.5-flash-lite-preview-06-17")
- `MAX_LABELS`: Maximum number of labels to apply (default: unlimited)
- `EXCLUDED_LABELS`: Comma-separated list of labels to exclude (default: "good-first-issue,help-wanted,wontfix,hack")

## Tekton Pipeline Integration

This package is designed to work with Tekton Pipelines as Code. Two PipelineRuns are provided:

### 1. PR CI Pipeline (`.tekton/pr-ci.yaml`)

**Trigger**: Automatically runs on all pull requests to the main branch

**Actions**:

- Clones the repository
- Runs PR linting checks
- Analyzes PR content with Gemini AI
- Applies appropriate labels automatically

**Usage**: No manual intervention needed - runs automatically on PR creation/updates.

### 2. Issue Creation Pipeline (`.tekton/issue-create.yaml`)

**Trigger**: Manual trigger via comment `/issue-create` on a PR

**Actions**:

- Clones the repository
- Analyzes PR content with Gemini AI
- Generates structured GitHub issue content
- Posts the generated issue as a PR comment

**Usage**: Comment `/issue-create` on any PR to generate an issue summary.

## Example Workflows

### Automatic PR Processing

1. Developer creates a PR
2. `pr-ci.yaml` pipeline automatically triggers
3. System validates PR title, description, and commits
4. Gemini analyzes the PR and suggests appropriate labels
5. Labels are automatically applied to the PR
6. Any linting issues are posted as PR comments

### Manual Issue Generation

1. Developer comments `/issue-create` on a PR
2. `issue-create.yaml` pipeline triggers
3. Gemini analyzes the PR content and generates a structured issue
4. Generated issue content is posted as a PR comment
5. Developer can copy/paste the content to create an actual GitHub issue

## AI Model Configuration

Both pipelines use Google Gemini AI for analysis. The model can be configured via the `GEMINI_MODEL` environment variable. The current default is `"gemini-2.5-flash-lite-preview-06-17"` which provides a good balance of speed and quality for PR analysis tasks.

## Development

### Dependencies

- Python 3.11+
- `requests` - GitHub API interactions
- `google-generativeai` - Gemini AI integration

### Local Development

```bash
cd hack/pr-ci
uv sync                    # Install dependencies
uv run pr-ci --help        # Test the CLI
```

## Security Considerations

- **GitHub Token**: Requires repo-level access for reading PR data and posting comments
- **Gemini API Key**: Used for AI analysis - ensure proper key rotation and access controls
- **Comment Markers**: Uses HTML comment markers to identify and manage automated comments
- **Input Validation**: All user inputs are validated before processing
