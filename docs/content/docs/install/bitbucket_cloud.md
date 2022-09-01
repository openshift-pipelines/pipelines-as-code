---
title: Bitbucket Cloud
weight: 14
---
# Use Pipelines-as-Code with Bitbucket Cloud

Pipelines-As-Code supports on [Bitbucket Cloud](https://bitbucket.org) through a webhook.

Follow the pipelines-as-code [installation](/docs/install/installation) according to your kubernetes cluster.

## Create Bitbucket Cloud App Password

Follow this guide to create an app password :

<https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/>

check these boxes to add the permissions to the token :

- Account: `Email`, `Read`
- Workspace membership: `Read`, `Write`
- Projects: `Read`, `Write`
- Issues: `Read`, `Write`
- Pull requests: `Read`, `Write`

NOTE: If you are going to configure webhook through CLI, you will need to also add additional permission

- Webhooks: `Read and write`

{{< hint info >}}
[Refer to this screenshot](/images/bitbucket-cloud-create-secrete.png) to verify you have properly configured the app password.
{{< /hint >}}

Keep the generated token noted somewhere, or otherwise you will have to recreate it.

## Setup Git Repository

There are 2 ways to set up the repository and configure the webhook:

### Setup Git Repository using tkn pac cli

- Use [`tkn pac setup bitbucket-cloud-webhook`](/docs/guide/cli) command which
will configure webhook and create repository CR.

You need to have a App Password created. tkn-pac will use this token to configure the webhook and add it in a secret
on cluster which will be used by pipelines-as-code controller for accessing the repository.

Below is the sample format for `tkn pac setup bitbucket-cloud-webhook`

```shell script
$ tkn pac setup bitbucket-cloud-webhook

✓ Setting up Bitbucket Webhook for Repository https://bitbucket.org/workspace/repo
? Please enter your bitbucket cloud username:  <username>
ℹ ️You now need to create a Bitbucket Cloud app password, please checkout the docs at https://is.gd/fqMHiJ for the required permissions
? Please enter the Bitbucket Cloud app password:  ************************************
? Please enter your controller public route URL:  <Pipeline As Code controller public URL>
✓ Webhook has been created on repository workspace/repo
? Would you like me to create the Repository CR for your git repository? Yes
? Please enter the namespace where the pipeline should run (default: repo-pipelines): 
! Namespace repo-pipelines is not found
? Would you like me to create the namespace repo-pipelines? Yes
✓ Repository workspace-repo has been created in repo-pipelines namespace
🔑 Webhook Secret workspace-repo has been created in the repo-pipelines namespace.
🔑 Repository CR workspace-repo has been updated with webhook secret in the repo-pipelines namespace
```

Alternatively, you could follow the [Setup Git Repository manually](#setup-git-repository-manually) guide to do it manually

### Setup Git Repository manually

- Go to you **“Repository settings“** tab on your **Repository** and click on the
  **Webhooks** tab and **“Add webhook“** button.

- Set a **Title** (i.e: Pipelines as Code)

- Set the *URL* to Pipeline as Code controller public URL. On OpenShift, you can get the public URL of the Pipelines-as-Code
  controller like this :

  ```shell
  echo https://$(oc get route -n pipelines-as-code pipelines-as-code-controller -o jsonpath='{.spec.host}')
  ```

- The individual events to select are :
  - Repository -> Push
  - Pull Request -> Created
  - Pull Request -> Updated
  - Pull Request -> Comment created

{{< hint info >}}
[Refer to this screenshot](/images/bitbucket-cloud-create-webhook.png) to make sure you have properly configured the Webhook.
{{< /hint >}}

- Click on *Save*

- You are now able to create a [`Repository CRD`](/docs/guide/repositorycrd)
  The repository CRD will have:

  - A **Username** (i.e: your Bitbucket username).
  - A reference to a kubernetes **Secret** containing the App Password as generated previously for Pipelines-as-Code operations.

- First create the secret with the app password in the `target-namespace`:

  ```shell
  kubectl -n target-namespace create secret generic bitbucket-cloud-token \
          --from-literal provider.token="APP_PASSWORD_AS_GENERATED_PREVIOUSLY"
  ```

- And then create the Repository CRD with the secret field referencing it, for example:

```yaml
  ---
  apiVersion: "pipelinesascode.tekton.dev/v1alpha1"
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: "https://bitbucket.com/workspace/repo"
    branch: "main"
    git_provider:
      user: "yourbitbucketusername"
      secret:
        name: "bitbucket-cloud-token"
        # Set this if you have a different key in your secret
        # key: “provider.token“
```

## Bitbucket Cloud Notes

- `git_provider.secret` cannot reference a secret in another namespace,
  Pipelines as code always assumes it will be the same namespace as where the
  repository has been created.

- `tkn-pac create` and `bootstrap` is not supported on Bitbucket Server.

{{< hint info >}}
You can only reference user by `ACCOUNT_ID` in owner file, see here for the
reasoning :

<https://developer.atlassian.com/cloud/bitbucket/bitbucket-api-changes-gdpr/#introducing-atlassian-account-id-and-nicknames>
{{< /hint >}}

{{< hint danger >}}

- There is no Webhook secret support in Bitbucket Cloud. To be able to secure
  the payload and not let a user hijack the CI, Pipelines-as-Code will fetch the
  ip addresses list from <https://ip-ranges.atlassian.com/> and enforce the
  webhook receptions comes only from the Bitbucket Cloud IPS.
- If you want to add some ips address or networks you can add them to the
  key **bitbucket-cloud-additional-source-ip** in the pipelines-as-code
  configmap in the pipelines-as-code namespace. You can add multiple
  network or ips separated by a comma.

- If you want to disable this behavior you can set the key
  **bitbucket-cloud-check-source-ip** to false in the pipelines-as-code
  configmap in the pipelines-as-code namespace.
{{< /hint >}}

## Update Token

When you have regenerated an app password you will need to  update it on cluster.
For example through the command line, you will want to replace `$password` and `$target_namespace` by their respective values:

You can find the secret name in Repository CR created.

  ```yaml
  spec:
    git_provider:
      secret:
        name: "bitbucket-cloud-token"
  ```

```shell
kubectl -n $target_namespace patch secret bitbucket-cloud-token -p "{\"data\": {\"provider.token\": \"$(echo -n $password|base64 -w0)\"}}"
```
