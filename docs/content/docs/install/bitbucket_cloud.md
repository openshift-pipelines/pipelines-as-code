---
title: Bitbucket Cloud
weight: 14
---
# Install Pipelines-As-Code for Bitbucket Cloud

Pipelines-As-Code has a full support on Bitbucket Cloud
(<https://bitbucket.org>) as Webhook.

After you have finished the [installation](/docs/install/installation) you can generate an app password for Pipelines-as-Code Bitbucket API operations.

Follow this guide to create an app password :

<https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/>

check these boxes to add the permissions to the token :

- Account: `Email`, `Read`
- Workspace membership: `Read`, `Write`
- Projects: `Read`, `Write`
- Issues: `Read`, `Write`
- Pull requests: `Read`, `Write`

{{< hint info >}}
[Refer to this screenshot](/images/bitbucket-cloud-create-secrete.png) to verify you have properly configured the app password.
{{< /hint >}}

Keep the generated token noted somewhere, or otherwise you will have to recreate it.

- Go to you **“Repository setting“** tab on your **Repository** and click on the
  **WebHooks** tab and **“Add webhook“** button.

- Set a **Title** (i.e: Pipelines as Code)

- Set the URL to the controller public URL. On OpenShift, you can get the public URL of the Pipelines-as-Code
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

- You are now able to create a [`Repository CRD`](/docs/guide/repositorycrd)
  The repository CRD will have:

  - A **Username** (i.e: your Bitbucket username).
  - A reference to a kubernetes **Secret** containing the App Password as generated previously for Pipelines-as-Code operations.

- First create the secret with the app password in the `target-namespace`:

  ```shell
  kubectl -n target-namespace create secret generic bitbucket-cloud-token \
          --from-literal “APP_PASSWORD_AS_GENERATED_PREVIOUSLY“
  ```

- And then create the Repository CRD with the secret field referencing it, for example:

```yaml
  ---
  apiVersion: “pipelinesascode.tekton.dev/v1alpha1“
  kind: Repository
  metadata:
    name: my-repo
    namespace: target-namespace
  spec:
    url: “https://bitbucket.com/workspace/repo“
    branch: “main“
    git_provider:
      user: “yourbitbucketusername“
      secret:
        name: “bitbucket-cloud-token“
        # Set this if you have a different key in your secret
        # key: “token“
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
