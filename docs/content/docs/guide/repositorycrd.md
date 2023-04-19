---
title: Repository CR
weight: 1
---
# Repository CR

The Repository CR serves the following purposes:

- Informing Pipelines as Code that an event from a specific URL needs to be handled.
- Specifying the namespace where the `PipelineRuns` will be executed.
- Referencing an API secret, username, or API URL if necessary for Git provider
  platforms that require it (e.g., when using webhooks instead of the GitHub
  application).
- Providing the last `PipelineRun`statuses for that repository (5 by default).
- Allowing for configuration of custom parameters within the `PipelineRun`that
  can be expanded based on certain filters.

The process involves creating a Repository CR inside the target namespace
my-pipeline-ci, using the tkn pac CLI or another method.

For example, this will create a Repo CR for the github repository
<https://github.com/linda/project>

```yaml
cat <<EOF|kubectl create -n my-pipeline-ci -f-
apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
kind: Repository
metadata:
  name: project-repository
spec:
  url: "https://github.com/linda/project"
EOF
```

With this configuration when an event from the `linda/project` repository
occurs, Pipelines as Code will know it needs to be handled and begin checking
out the contents of linda/project to match with the PipelineRun in the .tekton/
directory.

If the `PipelineRun` matches via its annotations the event, for example on a
specific branch and event like a `push` or `pull_request`. It wil start the
`PipelineRun` where the `Repository` CR has been created. You can only start the
`PipelineRun` in the namespace where the Repository CR is located.

## Additional Repository CR Security

An additional layer of security can be added by using a PipelineRun annotation
to explicitly target a specific namespace. However, a Repository CRD must still
be created in that namespace for it to be matched.

This annotation helps prevent bad actors on a cluster from hijacking
PipelineRun execution to a namespace they don't have access to. It let the user
specify the ownership of a repo matching the access of a specific namespace on
a cluster

To use this feature, add the following annotation to the pipeline:

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

Pipelines as Code will then only match the repository in the mynamespace
namespace instead of trying to match it from all available repositories on the
cluster.

{{< hint info >}}
Pipelines as Code installs a Kubernetes Mutating Admission Webhook to ensure
that only one Repository CRD is created per URL on a cluster.

If you disable this webhook, multiple Repository CRDs can be created for the
same URL. However, only the oldest created Repository CRD will be matched,
unless you use the `target-namespace` annotation.
{{< /hint >}}

## Concurrency

`concurrency_limit` allows you to define the maximum number of PipelineRuns running at any time for a Repository.

```yaml
spec:
  concurrency_limit: <number>
```

If there is multiple PipelineRuns matching the event, the PipelineRuns
that match the event will always be started in alphabetical order.

Example:

If you have three pipelineruns in a .tekton directory, and you create a pull
request with a `concurrency_limit` of 1 in the repository configuration, then all
of the pipelineruns will be executed in alphabetical order, one after the
other. At any given time, only one pipeline run will be in the running state,
while the rest will be queued.

## Custom Parameter Expansion

Using the `{{ param }}` syntax, Pipelines as Code let you expand a variable
inside a template directly within your PipelineRuns.

By default, there are
several variables exposed according to the event. To view all the variables
exposed by default, refer to the documentation on [Authoring
PipelineRuns](../authoringprs).

With the custom Parameter expansion, you can specify some custom values to be
replaced inside the template.

{{< hint warning >}}
Utilizing the Tekton PipelineRun parameters feature may generally be the
preferable approach, and custom params expansion should only be used in specific
scenarios where Tekton params cannot be used.
{{< /hint >}}

As an example here is a custom variable in the Repository CR `spec`:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
```

The variable name `{{ company }}` will be replaced by `My Beautiful Company`
anywhere inside your `PipelineRun` (including the remotely fetched task).

Alternatively, the value can be retrieved from a Kubernetes Secret.
For instance, the following code will retrieve the value for the company
`parameter` from a secret named `my-secret` and the key `companyname`:

```yaml
spec:
  params:
    - name: company
      secretRef:
        name: my-secret
        key: companyname
```

{{< hint info >}}

- If you have a `value` and a `secretRef` defined, the `value` will be used.
- If you don't have a `value` or a `secretRef` the parameter will not be
  parsed, it will be shown as `{{ param }}` in the `PipelineRun`.
- If you don't have a `name` in the `params` the parameter will not parsed.
- If you have multiple `params` with the same `name` the last one will be used.
{{< /hint >}}

### CEL filtering on custom parameters

You can define a `param` to only apply the custom parameters expansion when some
conditions has been matched on a `filter`:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
      filter:
        - name: event
          value: |
      pac.event_type == "pull_request"
```

The `pac` prefix contains all the values as set by default in the templates
variables. Refer to the [Authoring PipelineRuns](../authoringprs) documentation
for all the variable exposed by default.

The body of the payload is exposed inside the `body` prefix.

For example if you are running a Pull Request on Github pac will receive a
payload which has this kind of json:

```json
{
  "action": "opened",
  "number": 79,
  // .... more data
}
```

The filter can then do something like this:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
      filter:
        - name: event
          value: |
      body.action == "opened" && pac.event_type == "pull_request"
```

The payload of the event contains much more information that can be used with
the CEL filter. To see the specific payload content for your provider, refer to
the API documentation

You can have multiple `params` with the same name and different filters, the
first param that matches the filter will be picked up. This let you have
different output according to different event, and for example combine a push
and a pull request event.

{{< hint info >}}

- [Github Documentation for webhook events](https://docs.github.com/webhooks-and-events/webhooks/webhook-events-and-payloads?actionType=auto_merge_disabled#pull_request)
- [Gitlab Documentation for webhook events](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html)
{{< /hint >}}

## Scope GitHub token to a list of private and public repositories within and outside namespaces

The proposal helps user to scope GitHub token to a list of provided repositories which exist in same namespace(By providing configuration at Repository level)
as well as different namespaces(Global Configuration).

It is useful in a scenario where CI Repos Differ from CD Repos, and
the teams would like the generated GitHub Token from Pipelines As Code to allow control over these secondary repos,
even if they were not the one triggering the pipeline.

Ex:

Assuming CD repos are private and CI repo is the one which triggers the event, payload coming from CI repos fetches some tasks from CD Repos which are private.

There are two ways to scope GitHub token to a list of provided Repos

1. Scoping GH token to a list of Repos provided by global configuration
2. Scoping GH token to a list of Repos provided by Repository level configuration

### Prerequisite

Disable `secret-github-app-token-scoped` to `false` from `pipelines-as-code` configmap in order to scope
GitHub token to private and public repos listed under Global and Repo level configuration.

### Scoping GH token to a list of repos provided by global configuration

- When list of Repos provided by global configuration then scope all those repos by a Github token
irrespective of the namespaces.

- The configuration exists in `pipelines-as-code` configmap.

- The key which used to have list of Repos is `secret-github-app-scope-extra-repos`

  Ex:

  ```yaml
  apiVersion: v1
  data:
    secret-github-app-scope-extra-repos: "owner2/project2, owner3/project3"
  kind: ConfigMap
  metadata:
    name: pipelines-as-code
    namespace: pipelines-as-code
  ```

### Scoping GitHub token to a list of repos provided by Repository level configuration

- Scope token to a list of repos provided by `repo_list_to_scope_token` spec configuration within
the Repository custom resource

- Repos can be private or public

  ```yaml
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: test
    namespace: test-repo
  spec:
    url: "https://github.com/linda/project"
    repo_list_to_scope_token: 
    - "owner/project"
    - "owner1/project1"
  ```

  Now PAC will read `test` Repository custom resource and scope token to `owner/project`, `owner1/project1` and `linda/project` as well

  **Note:**

  1. Both `owner/project` and `owner1/project1` Repository should be in the same namespace where `test` Repository exists which is `test-repo` ns.

  2. If any one of the `owner/project` or `owner1/project1` doesn't exist then scoping token will fail

     Ex:

     If `owner1/project1` does not exist in the namespace

     Then below error will be displayed

     ```yaml
     repo owner1/project1 does not exist in namespace test-repo
     ```

#### Scenarios when both global and Repo level configurations provided

- When repos are provided by both `secret-github-app-scope-extra-repos` and `repo_list_to_scope_token`
then token will be scoped to all the repos from both configuration

    Ex:

  - List of Repos provided by `secret-github-app-scope-extra-repos` in cm

    ```yaml
    apiVersion: v1
    data:
      secret-github-app-scope-extra-repos: "owner2/project2, owner3/project3"
    kind: ConfigMap
    metadata:
      name: pipelines-as-code
      namespace: pipelines-as-code
    ```

  - List of Repos provided by `repo_list_to_scope_token` in Repository spec

    ```yaml
     apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
     kind: Repository
     metadata:
       name: test
       namespace: test-repo
     spec:
       url: "https://github.com/linda/project"
       repo_list_to_scope_token: 
       - "owner/project"
       - "owner1/project1"
    ```

    Now the GitHub token will be scoped to `owner/project`, `owner1/project1`, `owner2/project2`, `owner3/project3`, `linda/project`
