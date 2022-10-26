---
title: Repository CRD
weight: 1
---
# Repository CRD

The purposes of the Repository CRD  is:

- To let _Pipelines as Code_ know that this event from this URL needs to be handled.
- To let _Pipelines as Code_ know on which namespace the PipelineRuns are going to be executed.
- To reference an api secret, username or api URL if needed for the Git provider
  platforms that requires it (ie: when you are using webhooks method and not
  the GitHub application).
- To give the last Pipelinerun status for that Repository (5 by default).

The flow looks like this :

Using the tkn pac CLI or other method the user creates a `Repository` CR
inside the target namespace `my-pipeline-ci` :

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

Whenever there is an event coming from `github.com/linda/project` Pipelines as
Code will match it and starts checking out the content of the `linda/project`
for pipelinerun to match in the `.tekton/` directory.

The Repository CRD needs to be created in the namespace where Tekton Pipelines
associated with the source code repository would be executed, it cannot target
another namespace.

If there is multiples CRD matching the same event, only the oldest one will
match. If you need to match a specific namespace you would need to use the
target-namespace feature in the pipeline annotation (see below).

There is another optional layer of security where PipelineRun can have an
annotation to explicitly target a specific
namespace. It would still need to have a Repository CRD created in that
namespace to be able to be matched.

With this annotation a bad actor on a cluster cannot hijack the pipelineRun
execution to a namespace they don't have access to. To use that feature you
need to add this annotation to the pipeline annotation :

```yaml
pipelinesascode.tekton.dev/target-namespace: "mynamespace"
```

and Pipelines as Code will only match the repository in the mynamespace
Namespace rather than trying to match it from all available repository on cluster.

## Concurrency

`concurrency_limit` allows you to define the maximum number of PipelineRuns running at any time for a Repository.

```yaml
spec:
  concurrency_limit: <number>
```

Example:
Lets say you have 3 pipelines in `.tekton` directory, and you create a pull request with `concurrency_limit` defined as 1 in
Repository CR. Then all the pipelineruns will run one after the another, at any time only one pipelinerun would be in running
state and rest of them will be queued.
