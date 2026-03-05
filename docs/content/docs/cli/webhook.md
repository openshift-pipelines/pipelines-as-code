---
title: "webhook"
weight: 9
---

Use `tkn pac webhook` to add or update webhook secrets for your Git provider. You need this command when you are setting up a new webhook or rotating an existing provider token.

## Usage

```shell
tkn pac webhook add [-n namespace]
tkn pac webhook update-token [-n namespace]
```

## Add a Webhook Secret

`tkn pac webhook add` creates a new webhook secret for a given provider and updates the value in the existing `Secret` object that Pipelines-as-Code uses.

Supported providers: GitHub, GitLab, and Bitbucket Cloud.

## Update Provider Token

`tkn pac webhook update-token` updates the provider token for an existing `Secret` object that Pipelines-as-Code uses.
