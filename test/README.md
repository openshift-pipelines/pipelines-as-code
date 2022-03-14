# E2E Tests

The E2E Tests make sure we have the repositories getting updated if we have a repo with PAC installed on it.

It will checks if the repository have been updated at the end.

Most E2E tests has this basic flow :

- Create a temporary Namespace
- Create a Repository CR into it
- Create a Branch on a GitHUB repo
- Create a commmit with files like pipelinerun inside that branch, the pipelinerun with have the namespace annotation to
  force the repository match on the namespace we have created and not catching other CR that may matching it.
- Wait that the Repository is updated.
- Some other stuff are done directly on the eventlistenner sink, bypassing a bit the GitHUB apis and generating the
  webhook ourselves.

## Settings

here are all the variables that is used by the E2E tests.

- `TEST_GITHUB_API_URL` -- GitHub Api URL, needs to be set
- `TEST_GITHUB_TOKEN` -- Github token used to talk to the api url
- `TEST_GITHUB_REPO_OWNER` - The repo and owner (i.e: organization/repo)
- `TEST_GITHUB_REPO_INSTALLATION_ID` - The installation id when you have installed the repo on the app. (get it from the
  webhook event on the console)
- `TEST_EL_URL` - The eventlistenner public url, ingress or openshfit's route
- `TEST_EL_WEBHOOK_SECRET` - The webhook secret.
- `TEST_GITHUB_REPO_OWNER_WEBHOOK` - A repository/owner github repo that is configured with github webhooks.
- `TEST_BITBUCKET_CLOUD_API_URL` - Bitbucket Cloud Api URL: probably: `https://api.bitbucket.org/2.0`
- `TEST_BITBUCKET_CLOUD_USER` - Bitbucket Cloud user
- `TEST_BITBUCKET_CLOUD_E2E_REPOSITORY` - Bitbucket Cloud repository (ie: `project/repo`)
- `TEST_BITBUCKET_CLOUD_TOKEN` - Bitbucket Cloud token
- `TEST_GITLAB_API_URL` - Gitlab API URL i.e: `https://gitlab.com`
- `TEST_GITLAB_PROJECT_ID` - Gitlab project ID (you can get it in the repo details/settings)
- `TEST_GITLAB_TOKEN` - Gitlab Token

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
