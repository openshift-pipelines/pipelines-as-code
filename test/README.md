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

## Requirements

You need to have those env variable set :

- `TEST_GITHUB_API_URL` -- GitHub Api URL, needs to be set
- `TEST_GITHUB_TOKEN` -- Github token used to talk to the api url
- `TEST_GITHUB_REPO_OWNER` - The repo and owner (i.e: organization/repo)
- `TEST_GITHUB_REPO_INSTALLATION_ID` - The installation id when you have installed the repo on the app. (get it from the
  webhook event on the console)
- `TEST_EL_URL` - The eventlistenner public url, ingress or openshfit's route
- `TEST_EL_WEBHOOK_SECRET` - The webhook secret.

## Running

As long you have env variables set, you can just do a :

`make test-e2e`

and it will run the test-suite and cleans after itself,
