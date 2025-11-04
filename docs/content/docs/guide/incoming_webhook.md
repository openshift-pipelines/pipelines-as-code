---
title: Incoming Webhook
weight: 50
---

# Incoming webhook

Pipelines-as-Code supports the concept of incoming webhook URL. It lets you
trigger PipelineRuns in a Repository using a shared secret and URL,
instead of creating a new code iteration. This allows users to trigger
PipelineRuns using an HTTP request, e.g. with `curl` or from a webservice.

## Incoming Webhook URL

To use incoming webhooks in Pipelines-as-Code, you must configure the
incoming field in your Repository CRD. This field references a `Secret`, which
serves as the shared secret, as well as the branches targeted by the incoming
webhook. Once configured, Pipelines-as-Code will match `PipelineRuns` located in
your `.tekton` directory if the `on-event` annotation of the targeted PipelineRun is
targeting a push or incoming event.

{{< hint info >}}
If you are not using the GitHub app provider (i.e., webhook based provider) you
will need to have a `git_provider` spec to specify a token.

Additionally, since we are not able to detect automatically the type of provider
on URL, you will need to add it to the `git_provider.type` spec. Supported
values are:

- github
- gitlab
- bitbucket-cloud

Whereas for `github-apps` this does not need to be added.
{{< /hint >}}

### Required Parameters

Whether using the recommended POST request body or deprecated QueryParams,
the `/incoming` request accepts the following parameters:

| Parameter   | Type   | Description                                                                          | Required                                          |
|-------------|--------|--------------------------------------------------------------------------------------|---------------------------------------------------|
|`repository` |`string`| Name of Repository CR                                                                | `true`                                            |
|`namespace`  |`string`| Namespace with the Repository CR                                                     | When Repository name is not unique in the cluster |
|`branch`     |`string`| Branch configured for incoming webhook                                               | `true`                                            |
|`pipelinerun`|`string`| Name (or generateName) of PipelineRun, used to match PipelineRun definition          | `true`                                            |
|`secret`     |`string`| Secret key referenced by the Repository CR in desired incoming webhook configuration | `true`                                            |
|`params`     |`json`  | Parameters to override in PipelineRun context                                        | `false`                                           |

### GitHub App

The example below illustrates the use of GitHub App to trigger a PipelineRun
based on an incoming webhook URL.

The Repository Custom Resource (CR) specifies the target branch as
main and includes an incoming webhook URL with a shared password stored in a
Secret called `repo-incoming-secret`:

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  incoming:
    - targets:
        - main
      secret:
        name: repo-incoming-secret
      type: webhook-url
```

**Note:** If no secret key is specified in the Repository CR, the default key `secret` will be used to retrieve the secret value from the `repo-incoming-secret` Secret resource.

### Glob Pattern Matching in Targets

The `targets` field supports both exact string matching and glob patterns, allowing you to match multiple branches with a single rule.

**Glob patterns:** Use shell-style patterns:

- `*` - matches any characters (e.g., `feature/*` matches `feature/login`, `feature/api`)
- `?` - matches exactly one character (e.g., `v?` matches `v1`, `v2`)
- `[abc]` - matches one character from set (e.g., `[A-Z]*` matches any uppercase letter)
- `[0-9]` - matches digits (e.g., `v[0-9]*.[0-9]*` matches `v1.2`, `v10.5`)
- `{a,b,c}` - matches alternatives (e.g., `{dev,staging}/*` matches `dev/test` or `staging/test`)

**First-match-wins:** If multiple incoming webhooks match the same branch, the first matching webhook in the YAML order is used. Place more specific webhooks before general catch-all webhooks.

#### Examples

**Match feature branches with glob:**

```yaml
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  incoming:
    - targets:
        - "feature/*"  # Matches any branch starting with "feature/"
      secret:
        name: feature-webhook-secret
      type: webhook-url
```

**Multiple webhooks with first-match-wins:**

```yaml
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  incoming:
    # Production - checked first (most specific)
    - targets:
        - main
        - "v[0-9]*.[0-9]*.[0-9]*"  # Semver tags like v1.2.3
      secret:
        name: prod-webhook-secret
      params:
        - prod_env
      type: webhook-url

    # Feature branches - checked second
    - targets:
        - "feature/*"
        - "bugfix/*"
      secret:
        name: feature-webhook-secret
      params:
        - dev_env
      type: webhook-url

    # Catch-all - checked last
    - targets:
        - "*"  # Matches any branch not caught above
      secret:
        name: default-webhook-secret
      type: webhook-url
```

**Mix exact matches and glob patterns:**

```yaml
incoming:
  - targets:
      - main                            # Exact match
      - staging                         # Exact match
      - "release/v[0-9]*.[0-9]*.[0-9]*" # Semver releases
      - "hotfix/[A-Z]*-[0-9]*"          # JIRA tickets (e.g., JIRA-123, PROJ-456)
      - "{dev,test,qa}/*"               # Alternation pattern
    secret:
      name: repo-incoming-secret
    type: webhook-url
```

**Glob Pattern Syntax:**

- `*` - matches any characters (zero or more)
- `?` - matches exactly one character
- `[abc]` - matches one character: a, b, or c
- `[a-z]` - matches one character in range a to z
- `[0-9]` - matches one digit
- `{a,b,c}` - matches any of the alternatives (alternation)

**Best Practices:**

- Place production/sensitive webhooks first in the list
- Use exact matches for known branches when possible (faster than glob patterns)
- Use character classes `[0-9]`, `[A-Z]` for more precise matching
- Glob patterns match the entire branch name (no partial matches unless you use `*` prefix/suffix)
- Test your patterns: branch `feature-login` matches `feature-*` but not `*feature*`
- [Test your glob patterns online](https://www.digitalocean.com/community/tools/glob) before deploying to ensure they match only intended branches

### Using Incoming Webhooks

A PipelineRun is then annotated to target the incoming event and the main branch:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: target-pipelinerun
  annotations:
    pipelinesascode.tekton.dev/on-event: "[incoming]"
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
```

A secret called repo-incoming-secret is utilized as a shared password to ensure
that only authorized users can initiate the `PipelineRun`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-incoming-secret
  namespace: ns
type: Opaque
stringData:
  secret: very-secure-shared-secret
```

After setting this up, you will be able to start the PipelineRun with a POST
request sent to the controller URL appended with /incoming. The request
can include the very-secure-shared-secret, the repository name (repo), the
target branch (main), and the PipelineRun name either as URL query parameters
(legacy, insecure, and deprecated) or in the POST JSON body (recommended).

You can use the `generateName` field as the PipelineRun name but you will need
to make sure to specify the hyphen (-) at the end.

#### Legacy (URL query) method (deprecated)

```shell
curl -X POST 'https://control.pac.url/incoming?secret=very-secure-shared-secret&repository=repo&branch=main&pipelinerun=target-pipelinerun'
```

**Warning:** Passing secrets in the URL is insecure and will be deprecated. Please use the POST body method below.

#### Recommended (POST JSON body) method

```shell
curl -H "Content-Type: application/json" -X POST "https://control.pac.url/incoming" -d '{"repository":"repo","branch":"main","pipelinerun":"target-pipelinerun","secret":"very-secure-shared-secret"}'
```

In both cases, the `"/incoming"` path to the controller URL and the `"POST"` method will remain unchanged.

It is important to note that when the PipelineRun is triggered, Pipelines-as-Code will treat it as a push event and will have the capability to report the
status of the PipelineRuns. To obtain a report or a notification, a finally
task can be added directly to the Pipeline, or the Repo CRD can be inspected
using the tkn pac CLI. The [statuses](/docs/guide/statuses) documentation
provides guidance on how to achieve this.

### Passing dynamic parameter value to incoming webhook

You can define the value of any Pipelines-as-Code Parameters (including
redefining the [builtin ones](../authoringprs#default-parameters)).

You need to list the overridden or added params in the params section of the
Repo CR configuration and pass the value in the JSON body of the incoming webhook
request.

You will need to pass the `Content-Type` as `application/json` in the header of
your URL request.

Here is a Repository CR allowing passing the `pull_request_number` dynamic variable:

```yaml
---
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  incoming:
    - targets:
        - main
      params:
        - pull_request_number
      secret:
        name: repo-incoming-secret
      type: webhook-url
```

and here is a curl snippet passing the `pull_request_number` value:

```shell
curl -H "Content-Type: application/json" -X POST "https://control.pac.url/incoming" -d '{"repository":"repo","branch":"main","pipelinerun":"target-pipelinerun","secret":"very-secure-shared-secret","params": {"pull_request_number": "12345"}}'
```

The parameter value of `pull_request_number` will be set to `12345` when using
the variable `{{pull_request_number}}` in your PipelineRun.

### Using incoming webhook with GitHub Enterprise application

When using a GitHub application over to a GitHub Enterprise, you will need to
specify the `X-GitHub-Enterprise-Host` header when making the incoming webhook
request. For example when using curl:

```shell
curl -H "X-GitHub-Enterprise-Host: github.example.com" -X POST "https://control.pac.url/incoming?repository=repo&branch=main&secret=very-secure-shared-secret&pipelinerun=target-pipelinerun"
```

### Using incoming webhook with webhook based providers

Webhook based providers (i.e., GitHub Webhook, GitLab, Bitbucket etc..) support
incoming webhook, using the token provided in the git_provider section.

Here is an example of a Repository CRD matching the target branch main with a GitHub webhook provider:

```yaml
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: repo
  namespace: ns
spec:
  url: "https://github.com/owner/repo"
  git_provider:
    type: github
    secret:
      name: "owner-token"
  incoming:
    - targets:
        - main
      secret:
        name: repo-incoming-secret
      type: webhook-url
```

As noted in the section above, you need to specify an incoming secret inside
the `repo-incoming-secret` Secret.
