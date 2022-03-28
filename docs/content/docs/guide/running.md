---
title: Running the PipelineRun
weight: 4
---
# Running the PipelineRun

The user flow looks like this :

- A user create a `Pull Request` (or `Merge Request` in Gitlab).

- Pipelines as Code picks the event and matches to a Repo CRD installed on the
  cluster.

- The user is allowed to run the CI if :

  - The user is the owner of the repository.
  - The user is a collaborator on the repository.
  - The user is a public member on the organization of the repository.

- If the user is sending the Pull Request is inside an OWNER file located in the
  repository root on the main branch (the main branch as defined in the Github
  configuration for the repo) and added to either `approvers` or `reviewers`
  sections like this :

```yaml
approvers:
  - approved
```

then the user `approved` will be allowed.

- If the sender of a PR is not allowed to run CI but one of allowed user issue a
  `/ok-to-test` in any line of a comment the PR will be allowed to run CI.

- If the user is allowed, `Pipelines as Code` will start creating the
`PipelineRun` in the target user namespace.

- The user can follow the execution of your pipeline with the
[tkn](https://github.com/tektoncd/cli) cli :

```console
tkn pr logs -n my-pipeline-ci -Lf
```

Or with the OpenShift console inside your namespace you can follow the
pipelinerun execution via the URL provided on the "Checks" tab if you run with
Github App.
