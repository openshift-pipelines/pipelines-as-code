# Forgejo in a Pod for PAC

This will:

- spin a Forgejo deployment into a namespace
- create an ingress to a URL
- create a repository
- create a user/password
- create a token for user/password
- create an empty repository
- create a hook to go to a hook.pipelinesascode.com URL
- create a deployment with gosmee to forward smee URL to the pipelines as code controller
- create a repo crd to bind to it
- create a secret for the Git provider with the token generated previously

You can easily configure the script using environment variables, see the top of the files for the list.

Designed to run on kind.
