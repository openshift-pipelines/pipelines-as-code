# E2E Tests

The E2E Tests make sure we have the repositories getting updated if we have a repo with PAC installed on it.

It will checks if the repository have been updated at the end.

Most E2E tests has this basic flow :

- Create a temporary Namespace
- Create a Repository CR into it
- Create a Branch on a GitHUB repo
- Create a commit with files like pipelinerun inside that branch, the pipelinerun with have the namespace annotation to
  force the repository match on the namespace we have created and not catching other CR that may matching it.
- Wait that the Repository is updated.
- Some other stuff are done directly on the controller sink, bypassing a bit the GitHUB apis and generating the
  webhook ourselves.

## Settings

here are all the variables that is used by the E2E tests.

- `TEST_GITHUB_API_URL` -- GitHub Api URL, needs to be set i.e: `api.github.com`
- `TEST_GITHUB_TOKEN` -- Github token used to talk to the api url
- `TEST_GITHUB_REPO_OWNER` - The repo and owner (i.e: organization/repo)
- `TEST_GITHUB_REPO_OWNER_GITHUBAPP` - A repository/owner github repo that is configured with github apps.
- `TEST_GITHUB_REPO_INSTALLATION_ID` - The installation id when you have installed the repo on the app. (get it from the
  webhook event on the console)

**Hint:** Go to [Github Apps](https://github.com/settings/apps) (or *Settings > Developer settings > GitHub Apps*) choose the Github App and go to *Advanced > Recent Deliveries*
and search for **installation** which looks something like below

  ```yaml
    "installation": {
      "id": 29494069,
      "node_id": "MDIzOkludGVncmF0aW9uSW5zdGFsbGF0aW9uMjk0OTQwNjk="
    }
  ```

- `TEST_EL_URL` - The controller public url, ingress or openshfit's route
- `TEST_EL_WEBHOOK_SECRET` - The webhook secret.
- `TEST_GITHUB_REPO_OWNER_WEBHOOK` - A repository/owner github repo that is configured with github webhooks and
this repo should differ from the one which is configured as part of `TEST_GITHUB_REPO_OWNER_GITHUBAPP` env.
- `TEST_BITBUCKET_CLOUD_API_URL` - Bitbucket Cloud Api URL: probably: `https://api.bitbucket.org/2.0`
- `TEST_BITBUCKET_CLOUD_USER` - Bitbucket Cloud Username (you can get from "Personal Bitbucket settings" in UI)
- `TEST_BITBUCKET_CLOUD_E2E_REPOSITORY` - Bitbucket Cloud repository (i.e. `project/repo`)
- `TEST_BITBUCKET_CLOUD_TOKEN` - Bitbucket Cloud token
- `TEST_GITLAB_API_URL` - Gitlab API URL i.e: `https://gitlab.com`
- `TEST_GITLAB_PROJECT_ID` - Gitlab project ID (you can get it in the repo details/settings)
- `TEST_GITLAB_TOKEN` - Gitlab Token
- `TEST_GITEA_API_URL` - URL where GITEA is running (i.e: [GITEA_HOST](http://localhost:3000))
- `TEST_GITEA_SMEEURL` - URL of smee
- `TEST_GITEA_PASSWORD` - set password as **pac**
- `TEST_GITEA_USERNAME` - set username as **pac**
- `TEST_GITEA_REPO_OWNER` - set repo owner as **pac/pac**
- `TEST_BITBUCKET_SERVER_USER` - Bitbucket Data Center Username
- `TEST_BITBUCKET_SERVER_TOKEN` - Bitbucket Data Center token
- `TEST_BITBUCKET_SERVER_E2E_REPOSITORY` - Bitbucket Data Center repository (i.e. `project/repo`)
- `TEST_BITBUCKET_SERVER_API_URL` - URL where your Bitbucket Data Center instance is running.
- `TEST_BITBUCKET_SERVER_WEBHOOK_SECRET` - Webhook secret

You don't need to configure all of those if you restrict running your e2e tests to a subset.

## Running

As long you have env variables set, you can just do a :

`make test-e2e`

and it will run the test-suite and cleans after itself,

You can specify only a subsets of test to run with :

```shell
% cd test/; go test -tags=e2e -v -run TestGithub .
```

same goes for `TestGitlab` or other methods.

If you need to update the golden files in the end-to-end test, add the `-update` flag to the [go test](https://pkg.go.dev/cmd/go#hdr-Test_packages) command to refresh those files. First, run it if you expect the test output to change (or for a new test), then run it again without the flag to ensure everything is correct.

## Running nightly tests

Some tests are set as nightly which mean not run on every PR, because exposing rate limitation often.
We run those as nightly via github action on kind.

You can use `make test-e2e-nightly` if you want to run those manually as long
as you have all the env variables set.

If you are writing a test targeting a nightly test you need to check for the env variable:

```go
    if os.Getenv("NIGHTLY_E2E_TEST") != "true" {
        t.Skip("Skipping test since only enabled for nightly")
    }
```

and maybe add to the test-e2e-nightly Makefile target to the -run argument :

```bash
-run '(TestGithub|TestOtherPrefixOfTest)'
```

## Continuous Integration (CI) with GitHub Actions

Our E2E tests are automatically run as part of our CI pipeline using GitHub Actions. This section explains how the CI process works, the components involved, and how to troubleshoot or extend it.

### Overview

The E2E test CI pipeline runs on GitHub Actions using a Kind (Kubernetes in Docker) cluster. The workflow is defined in `.github/workflows/kind-e2e-tests.yaml` and uses helper scripts in `hack/gh-workflow-ci.sh`. Tests are executed against multiple provider categories to validate functionality across different Git providers.

The CI flow generally follows these steps:

1. Set up a Kind cluster
2. Install Pipelines as Code (PAC)
3. Configure necessary secrets
4. Run tests against different provider groups
5. Collect and store logs

### When Tests Run

Tests run on:

- Every Pull Request (PR) that modifies Go files
- As a nightly job at 05:00 UTC to detect regressions
- Manually via workflow dispatch (with optional debug capabilities)

### Test Categories

The tests are separated into two main categories (matrix strategy):

- `providers` - Tests for GitHub, GitLab, and Bitbucket
- `gitea_others` - Tests for Gitea and other non-provider specific functionality

This split helps reduce the load on external APIs during testing and provides more focused test results.

### Environment Variables

Tests rely heavily on environment variables for configuration. These are set at the job level in the workflow file and supplemented with secrets where needed. Some key variables include:

- Basic configuration: `KO_DOCKER_REPO`, `CONTROLLER_DOMAIN_URL`, etc.
- Provider-specific endpoints and credentials
- Test repository information
- Webhook configurations

Secrets are stored in GitHub Secrets and made available to the workflow via `${{ secrets.SECRET_NAME }}`.

### Helper Script

The `hack/gh-workflow-ci.sh` script contains several functions that assist in the CI process:

1. `create_pac_github_app_secret` - Creates the required secrets for GitHub app authentication
2. `create_second_github_app_controller_on_ghe` - Sets up a second controller for GitHub Enterprise
3. `run_e2e_tests` - Executes the E2E tests with proper filters
4. `collect_logs` - Gathers logs and diagnostic information

The script filters tests by category using pattern matching on test function names.

### Test Execution Flow

1. **Setup**:
   - Checkout code
   - Setup Go, ko, and gosmee client
   - Start Kind cluster
   - Install PAC controller

2. **Configuration**:
   - Create GitHub App secrets
   - Configure GHE environment (if needed)
   - Setup test environment variables

3. **Test Execution**:
   - Run selected tests against the specified provider category
   - For nightly runs, additional tests are included

4. **Artifacts Collection**:
   - Collect logs regardless of test outcome
   - Detect any panic in the controller logs
   - Upload artifacts to GitHub Actions

### Debugging CI

If a test fails in CI, you can:

1. Examine the workflow logs in GitHub Actions
2. Download the artifacts (logs) for detailed investigation
3. Use the "debug_enabled" option when manually triggering the workflow to get a tmate session

For local debugging, you can:

1. Set the same environment variables locally
2. Run `make test-e2e` with specific test filters

### Notifications

Failed nightly runs trigger Slack notifications to alert the team of potential regressions.

### Extending the CI

To add new provider tests:

1. Create test files following the existing patterns
2. Ensure proper naming convention to match the test category filters
3. Update environment variables if needed
4. For nightly-only tests, include the check for `NIGHTLY_E2E_TEST`

For infrastructure changes:

1. Modify the `kind-e2e-tests.yaml` workflow file
2. Update the `gh-workflow-ci.sh` helper script as needed
3. Test changes using workflow dispatch before merging

### Performance Considerations

1. Tests are configured with concurrency limits to prevent overlapping runs
2. The matrix strategy allows parallelization across provider categories
3. Rate limiting is managed by separating frequently-run tests from nightly tests
