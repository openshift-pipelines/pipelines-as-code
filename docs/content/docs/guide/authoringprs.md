---
title: Authoring PipelineRun
weight: 3
---

# Authoring PipelineRuns in `.tekton/` directory

- Pipelines-as-Code will always try to be as close to the Tekton template as
  possible. Usually, you will write your template and save them with a `.yaml`
  extension, and Pipelines-as-Code will run them.

- The `.tekton` directory must be at the top level of the repo.
  You can reference YAML files in other repos using remote URLs
  (see [Remote HTTP URLs](./resolver.md#remote-http-url) for more information),
  but PipelineRuns will only be triggered by events in the repository containing
  the `.tekton` directory.

- Using its [resolver](../resolver/) Pipelines-as-Code will try to bundle the
  PipelineRun with all its Tasks as a single PipelineRun with no external
  dependencies.

- Inside your pipeline, you need to be able to check out the commit as
  received from the webhook by checking out the repository from that ref. Most of the time
  you want to reuse the
  [git-clone](https://github.com/tektoncd-catalog/git-clone/tree/main/task/git-clone)
  task from the [tektoncd/catalog](https://github.com/tektoncd-catalog/git-clone/tree/main/task/git-clone).

- To be able to specify parameters of your commit and URL, Pipelines-as-Code
  gives you some “dynamic” variables that are defined according to the execution
  of the events. Those variables look like this `{{ var }}` and can be used
  anywhere in your template, see [below](#dynamic-variables) for the list of
  available variables.

- For Pipelines-as-Code to process your `PipelineRun`, you must have either an
  embedded `PipelineSpec` or a separate `Pipeline` object that references a YAML
  file in the `.tekton` directory. The Pipeline object can include `TaskSpecs`,
  which may be defined separately as Tasks in another YAML file in the same
  directory. It's important to give each `PipelineRun` a unique name to avoid
  conflicts. **PipelineRuns with duplicate names will never be matched**.

## Dynamic variables

Here is a list of all the dynamic variables available in Pipelines-as-Code. The
one that would be the most important to you would probably be the `revision` and `repo_url`
variables, they will give you the commit SHA and the repository URL that is
getting tested. You usually use this with the
[git-clone](https://hub.tekton.dev/tekton/task/git-clone) task to be able to
check out the code that is being tested.

| Variable            | Description                                                                                                                                                                     | Example                             | Example Output                                                                                                                                                |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| body                | The full payload body (see [below](#using-the-body-and-headers-in-a-pipelines-as-code-parameter))                                                                               | `{{body.pull_request.user.email }}` | <email@domain.com>                                                                                                                                            |
| event_type          | The event type (eg: `pull_request` or `push`)                                                                                                                                   | `{{event_type}}`                    | pull_request          (see the note for GitOps Comments [here]({{< relref "/docs/guide/gitops_commands.md#event-type-annotation-and-dynamic-variables" >}}) ) |
| git_auth_secret     | The secret name auto-generated with provider token to check out private repos.                                                                                                  | `{{git_auth_secret}}`               | pac-gitauth-xkxkx                                                                                                                                             |
| headers             | The request headers (see [below](#using-the-body-and-headers-in-a-pipelines-as-code-parameter))                                                                                 | `{{headers['x-github-event']}}`     | push                                                                                                                                                          |
| pull_request_number | The pull or merge request number, only defined when we are in a `pull_request` event or push event occurred when pull request is merged.                                        | `{{pull_request_number}}`           | 1                                                                                                                                                             |
| repo_name           | The repository name.                                                                                                                                                            | `{{repo_name}}`                     | pipelines-as-code                                                                                                                                             |
| repo_owner          | The repository owner.                                                                                                                                                           | `{{repo_owner}}`                    | openshift-pipelines                                                                                                                                           |
| repo_url            | The repository full URL.                                                                                                                                                        | `{{repo_url}}`                      | https:/github.com/repo/owner                                                                                                                                  |
| revision            | The commit full sha revision.                                                                                                                                                   | `{{revision}}`                      | 1234567890abcdef                                                                                                                                              |
| sender              | The sender username (or account ID on some providers) of the commit.                                                                                                            | `{{sender}}`                        | johndoe                                                                                                                                                       |
| source_branch       | The branch name where the event comes from.                                                                                                                                     | `{{source_branch}}`                 | main                                                                                                                                                          |
| git_tag             | The Git tag pushed (only available for tag push events; otherwise empty `""`).                                                                                                  | `{{git_tag}}`                       | v1.0                                                                                                                                                          |
| source_url          | The source repository URL from where the event comes (same as the value `repo_url` for push events).                                                                            | `{{source_url}}`                    | https:/github.com/repo/owner                                                                                                                                  |
| target_branch       | The branch name on which the event targets (same as `source_branch` for push events).                                                                                           | `{{target_branch}}`                 | main                                                                                                                                                          |
| target_namespace    | The target namespace where the Repository has matched and the PipelineRun will be created.                                                                                      | `{{target_namespace}}`              | my-namespace                                                                                                                                                  |
| trigger_comment     | The comment triggering the PipelineRun when using a [GitOps command]({{< relref "/docs/guide/running.md#gitops-command-on-pull-or-merge-request" >}}) (like `/test`, `/retest`) | `{{trigger_comment}}`               | /merge-pr branch                                                                                                                                              |
| pull_request_labels | The labels of the pull request separated by a newline                                                                                                                           | `{{pull_request_labels}}`           | bugs\nenhancement                                                                                                                                             |

Note: When using the `{{ pull_request_number }}` variable in a push-triggered PipelineRun when a pull request is merged and the commit is associated with multiple pull requests
the git provider API may return more than one pull request. In such cases, the `{{ pull_request_number }}` variable will contain the number of the first pull request returned by the API.

The `{{ pull_request_number }}` variable is currently supported only for the GitHub provider when used in a push event.

### Defining Parameters with Object Values in YAML

When working with YAML, particularly when defining parameters, you might encounter situations where you need to pass an object or a dynamic variable (e.g., `{{ body }}`) as the value of a parameter. However, YAML's validation rules prevent such values from being defined inline.

For instance, if you attempt to define a parameter like this:

```yaml
spec:
  params:
    - name: body
      value: {{ body }}  # This will result in a YAML validation error
  pipelineSpec:
    tasks:
```

You will encounter a YAML validation error because objects or multiline strings cannot be placed inline. To resolve this issue and ensure your YAML is correctly validated, you should define the value in block format instead of inline. Here’s an example:

```yaml
spec:
  params:
    - name: body
      value: |-
        {{ body }}
    # Alternatively, use '>' to specify that the value will be in block format
    - name: pull_request
      value: >
        {{ body.pull_request }}
  pipelineSpec:
    tasks:
```

By using the block format, you can avoid validation errors and ensure that your YAML is properly structured.

## Matching an event to a PipelineRun

Each `PipelineRun` can match different Git provider events through some special
annotations on the `PipelineRun`.

For example, when you have these metadata in
your `PipelineRun`:

```yaml
metadata:
  name: pipeline-pr-main
annotations:
  pipelinesascode.tekton.dev/on-target-branch: "[main]"
  pipelinesascode.tekton.dev/on-event: "[pull_request]"
```

`Pipelines-as-Code` will match the PipelineRun `pipeline-pr-main` if the Git
provider events target the branch `main` and it's coming from a `[pull_request]`

There are many ways to match an event to a PipelineRun, head over to this patch
[page]({{< relref "/docs/guide/matchingevents.md" >}}) for more details.

## Using the body and headers in a Pipelines-as-Code parameter

Pipelines-as-Code lets you access the full body and headers of the request as a CEL expression.

This allows you to go beyond the standard variables and even play with multiple
conditions and variables to output values.

For example, if you want to get the title of the Pull Request in your PipelineRun you can simply access it like this:

```go
{{ body.pull_request.title }}
```

You can then get creative and for example mix the variable inside a python
script to evaluate the json.

This task, for example, is using python and will check the labels on the PR,
`exit 0` if it has the label called 'bug' on the pull request or `exit 1` if it
doesn't:

```yaml
taskSpec:
  steps:
    - name: check-label
      image: registry.access.redhat.com/ubi9/ubi
      script: |
        #!/usr/bin/env python3
        import json
        labels=json.loads("""{{ body.pull_request.labels }}""")
        for label in labels:
            if label['name'] == 'bug':
              print('This is a PR targeting a BUG')
              exit(0)
        print('This is not a PR targeting a BUG :(')
        exit(1)
```

The expressions are CEL expressions so you can as well make some conditional:

```yaml
- name: bash
  image: registry.access.redhat.com/ubi9/ubi
  script: |
    if {{ body.pull_request.state == "open" }}; then
      echo "PR is Open"
    fi
```

if the PR is open the condition then returns `true` and the shell script sees this
as a valid boolean.

Headers from the payload body can be accessed from the `headers` keyword, note that headers are case-sensitive,
for example, this will show the GitHub event type for a GitHub event:

```yaml
{{ headers['X-Github-Event'] }}
```

and then you can do the same conditional or access as described above for the `body` keyword.

## Using the temporary GitHub APP Token for GitHub API operations

You can use the temporary installation token that is generated by Pipelines as
Code from the GitHub App to access the GitHub API.

The token value is stored in the temporary git-auth secret as generated for [private
repositories](../privaterepo/) in the key `git-provider-token`.

As an example, if you want to add a comment to your pull request, you can use the
[github-add-comment](https://hub.tekton.dev/tekton/task/github-add-comment)
task from the [Tekton Hub](https://hub.tekton.dev)
using a [pipelines as code annotation](../resolver/#remote-http-url):

```yaml
pipelinesascode.tekton.dev/task: "github-add-comment"
```

you can then add the task to your [tasks section](https://tekton.dev/docs/pipelines/pipelines/#adding-tasks-to-the-pipeline) (or [finally](https://tekton.dev/docs/pipelines/pipelines/#adding-finally-to-the-pipeline) tasks) of your PipelineRun :

```yaml
[...]
tasks:
  - name:
      taskRef:
        name: github-add-comment
      params:
        - name: REQUEST_URL
          value: "{{ repo_url }}/pull/{{ pull_request_number }}"
        - name: COMMENT_OR_FILE
          value: "Pipelines-as-Code IS GREAT!"
        - name: GITHUB_TOKEN_SECRET_NAME
          value: "{{ git_auth_secret }}"
        - name: GITHUB_TOKEN_SECRET_KEY
          value: "git-provider-token"
```

Since we are using the dynamic variables we are able to reuse this on any
PullRequest from any repositories.

and for completeness, here is another example of how to set the GITHUB_TOKEN
environment variable on a task step:

```yaml
env:
  - name: GITHUB_TOKEN
    valueFrom:
      secretKeyRef:
        name: "{{ git_auth_secret }}"
        key: "git-provider-token"
```

{{< hint info >}}

- On GitHub apps the generated installation token [will be available for 8 hours](https://docs.github.com/en/developers/apps/building-github-apps/refreshing-user-to-server-access-tokens)
- On GitHub apps the token is scoped to the repository the event (payload) comes
  from unless [configured](/docs/install/settings#pipelines-as-code-configuration-settings) differently on the cluster.

{{< /hint >}}

## Example

`Pipelines as code` test itself, you can see the examples in its
[.tekton](https://github.com/openshift-pipelines/pipelines-as-code/tree/main/.tekton) repository.
