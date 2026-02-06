---
name: e2e-investigate
description: >-
  This skill should be used when the user asks to "investigate e2e failure",
  "debug e2e test", "analyze e2e logs", "what failed in e2e", "check e2e artifacts",
  or needs help understanding why an E2E test failed. Provides systematic workflow
  for correlating logs across controller, webhook, watcher, and Kubernetes events.
version: 0.1.0
---

# E2E Test Failure Investigation

This skill provides a systematic workflow for investigating E2E test failures using downloaded GitHub Actions artifacts.

## Step 1: Locate Artifacts

First, determine where the artifacts are located. Use AskUserQuestion with these options:

1. **I have artifacts downloaded** - Ask for the path (typically `tmp/e2e/logs-e2e-tests-*`)
2. **Download from GitHub Actions** - Ask which pattern to download
3. **Extract from zip in Downloads** - Use the download script

### Available Test Patterns

- `flaky` - Flaky test suite
- `github_1`, `github_2` - GitHub provider tests
- `gitea_1`, `gitea_2`, `gitea_3` - Gitea provider tests
- `github_second_controller` - Second controller tests
- `gitlab_bitbucket` - GitLab and Bitbucket tests
- `concurrency` - Concurrency tests

### Downloading Artifacts

If artifacts need to be downloaded, use one of these methods:

```bash
# Using gh CLI directly
gh run download --pattern "logs-e2e-tests-github_1-*" -D tmp/e2e/

# Or run the download script (see references/download-script.sh)
bash .claude/skills/e2e-investigate/references/download-script.sh
```

The artifacts will be in: `tmp/e2e/logs-e2e-tests-{pattern}-{timestamp}/`

## Step 2: Find Failed Tests

Read `e2e-test-output.log` and extract failures:

```bash
grep -B10 -- "--- FAIL.*Test" e2e-test-output.log
```

This shows the 10 lines before each test failure, capturing:

- The test name (e.g., `TestGithubPullRequest`)
- Error messages and assertions
- The namespace used (e.g., `pac-e2e-ns-xxxxx`)

### Extract Key Information

From each failure, note:

1. **Test name**: The function name after `--- FAIL:`
2. **Namespace**: Look for `pac-e2e-ns-` pattern in the output
3. **Error type**: Timeout, assertion failure, resource error, etc.

## Step 3: Correlate with Component Logs

Search the component logs for the namespace or error:

### Controller Logs

```bash
grep "pac-e2e-ns-xxxxx" pipelines-as-code-controller.log | head -100
```

Look for:

- Error messages with `"level":"error"`
- Reconcile failures
- API rate limiting
- Authentication issues

### Watcher Logs

```bash
grep "pac-e2e-ns-xxxxx" pipelines-as-code-watcher.log
```

Look for:

- PipelineRun status changes
- Completion/failure events

### Webhook Logs

```bash
grep "pac-e2e-ns-xxxxx" pipelines-as-code-webhook.log
```

Look for:

- Incoming webhook payloads
- Validation errors
- Parsing failures

## Step 4: Check Kubernetes Resources

### Events

```bash
grep -i "error\|failed\|warning" events
```

Look for:

- Pod scheduling issues
- Resource quota problems
- Image pull failures

### PipelineRuns

Read `pac-pipelineruns.yaml` and search for:

- `status: False` conditions
- `reason: Failed`
- The specific namespace

### Repositories

Check `pac-repositories.yaml` for:

- Configuration issues
- Secret references

## Step 5: Check API and Webhook Issues

### API Instrumentation

Look in `api-instrumentation/` for per-test JSON files:

- Rate limit errors
- Authentication failures
- API response errors

### Gosmee Webhook Logs

Check `gosmee/main.log` for:

- Webhook delivery failures
- Replay issues
- Connection problems

## Step 6: Present Findings

Summarize your findings with:

1. **Test that failed**: Name and basic error
2. **Root cause**: What actually went wrong
3. **Relevant log excerpts**: Key lines from component logs
4. **Resources affected**: PipelineRuns, pods, etc.
5. **Suggested fix**: If apparent from the logs

## Reference Files

See the `references/` directory for:

- `log-correlation.md` - How to trace events across logs
- `common-failures.md` - Known failure patterns and fixes
- `download-script.sh` - Script to download artifacts
