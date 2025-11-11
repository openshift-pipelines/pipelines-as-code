---
title: GitHub Webhook
weight: 12
---

# Use Pipelines-as-Code with GitHub Webhook

If you are not able to create a GitHub application, you can use Pipelines-as-Code with [GitHub Webhook](https://docs.github.com/en/developers/webhooks-and-events/webhooks/creating-webhooks) on your repository.

Using Pipelines-as-Code through GitHub webhook does not give you access to the
[GitHub CheckRun
API](https://docs.github.com/en/rest/guides/getting-started-with-the-checks-api),
therefore the status of
the tasks will be added as a Comment on the PullRequest and not through the **Checks** Tab.

GitOps comments (i.e., /retest /ok-to-test) with GitHub webhook are
not supported. If you need to restart the CI, you will need to generate a new
commit. You can make it quick with this command line snippet (adjust branchname to the name of
the branch):

```console
git commit --amend -a --no-edit && git push --force-with-lease origin branchname
```

## Create GitHub Personal Access Token

After Pipelines-as-Code [installation](/docs/install/installation), you will
need to create a GitHub personal access token for Pipelines-as-Code GitHub API
operations.

Follow this guide to create a personal token:

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token>

### [Fine-grained token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token#creating-a-fine-grained-personal-access-token)

If you want to generate a fine-grained token (which is more secure), you can
scope your token to the repository you want tested.

The permissions needed are:

| Name            | Access         |
|:---------------:|:--------------:|
| Administration  | Read Only      |
| Metadata        | Read Only      |
| Content         | Read Only      |
| Commit statuses | Read and Write |
| Pull request    | Read and Write |
| Webhooks        | Read and Write |

### [Classic Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token#creating-a-personal-access-token-classic)

Depending on the Repository access scope, the token will need different
permissions. For public repositories, the scope needed is:

* `public_repo` scope

For private repositories, the scope needed is:

* The whole `repo` scope

{{< hint info >}}
You can click directly on this link to prefill the permissions needed:
<https://github.com/settings/tokens/new?description=pipelines-as-code-token&scopes=repo>
{{< /hint >}}

You will have to note the generated token somewhere, or you will have to recreate it.

For best security practice, you will probably want to have a short token
expiration (like the default 30 days). GitHub will send you a notification email
if your token expires. Follow [Update Token](#update-token) to replace an expired token with a new one.

**NOTE:** If you are going to configure a webhook through CLI, you must also add the scope `admin:repo_hook`.

## Create a `Repository` and configure webhook

There are two ways to create the `Repository` and configure the webhook:

### Create a `Repository` and configure webhook using the `tkn pac` tool

* Use the [`tkn pac create repo`](/docs/guide/cli) command to
  configure a webhook and create the `Repository` CR.

  You need to have a personal access token created with the `admin:repo_hook` scope. `tkn pac` will use this token to configure the
  webhook, and add it to a secret in the cluster which will be used by the Pipelines-as-Code controller for accessing the `Repository`.
  After configuring the webhook, you will be able to update the token in the secret with just the scopes mentioned [here](#create-github-personal-access-token).

Below is the sample format for `tkn pac create repo`

```shell script
$ tkn pac create repo

? Enter the Git repository url (default: https://github.com/owner/repo):
? Please enter the namespace where the pipeline should run (default: repo-pipelines):
! Namespace repo-pipelines is not found
? Would you like me to create the namespace repo-pipelines? Yes
✓ Repository owner-repo has been created in repo-pipelines namespace
✓ Setting up GitHub Webhook for Repository https://github.com/owner/repo
👀 I have detected a controller url: https://pipelines-as-code-controller-openshift-pipelines.apps.awscl2.aws.ospqa.com
? Do you want me to use it? Yes
? Please enter the secret to configure the webhook for payload validation (default: sJNwdmTifHTs):  sJNwdmTifHTs
ℹ️ You now need to create a GitHub personal access token; please check the docs at https://is.gd/KJ1dDH for the required scopes
? Please enter the GitHub access token:  ****************************************
✓ Webhook has been created on repository owner/repo
🔑 Webhook Secret owner-repo has been created in the repo-pipelines namespace.
🔑 Repository CR owner-repo has been updated with webhook secret in the repo-pipelines namespace
ℹ Directory .tekton has been created.
✓ We have detected your repository using the programming language Go.
✓ A basic template has been created in /home/Go/src/github.com/owner/repo/.tekton/pipelinerun.yaml, feel free to customize it.
ℹ You can test your pipeline by pushing the generated template to your git repository

```

### Create a `Repository` and configure webhook manually

* Go to your repository or organization **Settings** --> **Webhooks** and click on the **Add webhook** button.

  * Set the **Payload URL** to the Pipelines-as-Code controller public URL. On OpenShift, you can get the public URL of the Pipelines-as-Code controller like this:

    ```shell
    echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
    ```

  * Choose Content type as **application/json**

  * Add a Webhook secret or generate a random one with this command (and note it; we will need it later):

    ```shell
    head -c 30 /dev/random | base64
    ```

  * Click "Let me select individual events" and select these events:
    * Commit comments
    * Issue comments
    * Pull request
    * Pushes

    {{< hint info >}}
    [Refer to this screenshot](/images/pac-direct-webhook-create.png) to verify you have properly configured the webhook.
    {{< /hint >}}

  * Click on **Add webhook**

* You can now create a [`Repository CRD`](/docs/guide/repositorycrd).
  It will have:

  A reference to a Kubernetes **Secret** containing the Personal token as generated previously and another reference to a Kubernetes **Secret** to validate the webhook payload as set previously in your webhook configuration.

* Create the `Secret` with the personal token and webhook secret in the `target-namespace` (where you are planning to run your pipeline CI):

  ```shell
  kubectl -n target-namespace create secret generic github-webhook-config \
    --from-literal provider.token="TOKEN_AS_GENERATED_PREVIOUSLY" \
    --from-literal webhook.secret="SECRET_AS_SET_IN_WEBHOOK_CONFIGURATION"
  ```

* Create the `Repository CRD` referencing everything:

  ```yaml
  ---
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: "https://github.com/owner/repo"
    git_provider:
      secret:
        name: "github-webhook-config"
        # Set this if you have a different key in your secret
        # key: "provider.token"
      webhook_secret:
        name: "github-webhook-config"
        # Set this if you have a different key for your secret
        # key: "webhook.secret"
  ```

## GitHub Webhook Notes

* Pipelines-as-Code always assumes that the `Secret` is in the same namespace where the `Repository` has been created.

## Add Webhook Secret

* For an existing `Repository`, if the webhook secret has been deleted (or you want to add a new webhook to project settings) for GitHub,
  use the `tkn pac webhook add` command to add a webhook to project repository settings, as well as update the `webhook.secret`
  key in the existing `Secret` object without updating the `Repository`.

Below is the sample format for `tkn pac webhook add`:

```shell script
$ tkn pac webhook add -n repo-pipelines

✓ Setting up GitHub Webhook for Repository https://github.com/owner/repo
👀 I have detected a controller url: https://pipelines-as-code-controller-openshift-pipelines.apps.awscl2.aws.ospqa.com
? Do you want me to use it? Yes
? Please enter the secret to configure the webhook for payload validation (default: AeHdHTJVfAeH):  AeHdHTJVfAeH
✓ Webhook has been created on repository owner/repo
🔑 Secret owner-repo has been updated with webhook secret in the repo-pipelines namespace.

```

**Note:** If the `Repository` exists in a namespace other than the `default` namespace, use `tkn pac webhook add [-n namespace]`.
  In the above example, the `Repository` exists in the `repo-pipelines` namespace rather than the `default` namespace; therefore
  the webhook was added in the `repo-pipelines` namespace.

## Update Token

There are two ways to update the provider token for the existing `Repository`:

### Update using tkn pac CLI

* Use the [`tkn pac webhook update-token`](/docs/guide/cli) command which
  will update provider token for the existing `Repository` CR.

Below is the sample format for `tkn pac webhook update-token`:

```shell script
$ tkn pac webhook update-token -n repo-pipelines

? Please enter your personal access token:  ****************************************
🔑 Secret owner-repo has been updated with new personal access token in the repo-pipelines namespace.

```

**NOTE:** If the `Repository` exists in a namespace other than the `default` namespace, use `tkn pac webhook update-token [-n namespace]`.
  In the above example, the `Repository` exists in the `repo-pipelines` namespace rather than the `default` namespace; therefore
  the webhook token was updated in the `repo-pipelines` namespace.

### Update by changing `Repository` YAML or using `kubectl patch` command

When you have regenerated a new token, you must update it in the cluster.
For example, you can replace `$NEW_TOKEN` and `$target_namespace` with their respective values:

You can find the secret name in the `Repository` CR.

  ```yaml
  spec:
    git_provider:
      secret:
        name: "github-webhook-config"
  ```

```shell
kubectl -n $target_namespace patch secret github-webhook-config -p "{\"data\": {\"provider.token\": \"$(echo -n $NEW_TOKEN|base64 -w0)\"}}"
```
