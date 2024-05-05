---
title: Azure Devops
weight: 17
---
# Use Pipelines-as-Code with Azure Devops

Pipelines-As-Code supports on [Azure Devops](https://azure.microsoft.com/en-us/products/devops) through a webhook.

Follow the Pipelines-As-Code [installation](/docs/install/installation) according to your Kubernetes cluster.

* You will have to generate a personal token as the manager of the Project,
  follow the steps here:

<https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=Windows>

The token will need to have atleast `Read, write, & manage` permissions under `code`.

You may want to note somewhere the generated token, or otherwise you will have to
recreate it.

* Create a Webhook on the repository following this guide :
<https://learn.microsoft.com/en-us/azure/devops/service-hooks/services/webhooks?view=azure-devops>

Provide the header value of service hook based on required event type

| Event Type   |  Header Value |
| ----------|---------|
| Code Pushed  | X-Azure-DevOps-EventType:git.push |
| Pull request created | X-Azure-DevOps-EventType:git.pullrequest.created|
| Pull request updated | X-Azure-DevOps-EventType:git.pullrequest.updated|
| Pull request commented on| X-Azure-DevOps-EventType:git.pullrequest.comment|

* Create a secret with personal token in the `target-namespace`

  ```shell
  kubectl -n target-namespace create secret generic webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
  ```

* And finally create Repository CRD with the secret field referencing it.

  * Here is an example of a Repository CRD :

```yaml
  ---
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: 'https://dev.azure.com/YOUR_ORG_NAME/YOUR_PROJ_NAME/_git/YOUR_REPO_NAME'
    git_provider:
      secret:
        name: "webhook-config"
        # Set this if you have a different key in your secret
        # key: "provider.token"
```
