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
- `TEST_BITBUCKET_SERVER_USER` - Bitbucket Server Username
- `TEST_BITBUCKET_SERVER_TOKEN` - Bitbucket Server token
- `TEST_BITBUCKET_SERVER_E2E_REPOSITORY` - Bitbucket Server repository (i.e. `project/repo`)
- `TEST_BITBUCKET_SERVER_API_URL` - URL where your Bitbucket Server instance is running.
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
