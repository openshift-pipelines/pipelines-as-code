# Pipelines-as-Code 🚀

[![Container Repository on GHCR](https://img.shields.io/badge/GHCR-image-87DCC0.svg?logo=GitHub)](https://github.com/openshift-pipelines/pipelines-as-code/pkgs/container/pipelines-as-code)
[![codecov](https://codecov.io/gh/openshift-pipelines/pipelines-as-code/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift-pipelines/pipelines-as-code)
[![Go Report Card](https://goreportcard.com/badge/google/ko)](https://goreportcard.com/report/openshift-pipelines/pipelines-as-code)
[![E2E Tests](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/kind-e2e-tests.yaml/badge.svg)](https://github.com/openshift-pipelines/pipelines-as-code/actions/workflows/kind-e2e-tests.yaml)

Pipelines-as-Code is an opinionated CI/CD solution built on OpenShift Pipelines
and Tekton. It enables you to define, manage, and execute pipelines directly
from your source code repository.

📖 **Full documentation:** [pipelinesascode.com](https://pipelinesascode.com)  
🛠️ **Development branch docs:** [here](https://nightly.pipelines-as-code.pages.dev/)

---

## 🚀 Introduction

Pipelines-as-Code follows the [Pipelines-as-Code
methodology](https://teamhub.com/blog/understanding-pipeline-as-code-in-software-development/),
bringing it directly to [Tekton](https://tekton.dev/) and [OpenShift Pipelines](https://docs.openshift.com/pipelines/latest/about/about-pipelines.html).

### 🎯 Key Features

- ✅ **Pull Request status support** – Automatically updates PR status during pipeline execution.
- 🔄 **GitHub Checks API** – Recheck and validate your pipelines effortlessly.
- 🔗 **GitHub PR & Commit Events** – Trigger pipelines via pull requests, pushes, and commits.
- 💬 **PR Actions in Comments** – Use `/retest` and other commands for better control.
- 📂 **Event-based Pipelines** – Define different pipelines for different Git events.
- ⚡ **Automatic Task Resolution** – Supports local tasks, Tekton Hub, and remote URLs.
- 📦 **Efficient Config Retrieval** – Uses GitHub blobs & objects API to fetch configs.
- 🔐 **Access Control** – Manage via GitHub orgs or Prow-style [`OWNER`](https://www.kubernetes.dev/docs/guide/owners/) files.
- 🛠️ **`tkn-pac` CLI Plugin** – Easily manage Pipelines-as-Code repositories.
- 🌍 **Multi-Git Support** – Works with GitHub (via GitHub App & Webhook), GitLab, Gitea, Bitbucket Data Center & Cloud via webhooks.

Head over to the Documentation for the full feature list and detailed guides:

<https://pipelinesascode.com>

---

## 🏁 Getting Started

The easiest way to get started is using the `tkn pac` CLI and its bootstrap command.

We have a full walk-through tutorial here:

<https://pipelinesascode.com/docs/install/getting-started/>

This guide will guide you through the installation process and help you set up
your first Pipelines-as-Code repository.

## 🤝 Contributing

We ❤️ contributions!

If you'd like to help improve Pipelines-as-Code, check out our contribution guide: [Contribute Here](https://pipelinesascode.com/docs/dev/).

---

## 💬 Getting in Touch

🔔 **Join the Community:**

- 📅 Subscribe to the [community calendar](https://calendar.google.com/calendar/embed?src=53eb8e69e3a902ea3a31fe6795f69df165d9bb22a8ab11ed5c9cbd27ee654742%40group.calendar.google.com)
- 💬 Chat on Slack: [#pipelinesascode](https://tektoncd.slack.com/archives/C04URDDJ9MZ) ([Join TektonCD Slack](https://github.com/tektoncd/community/blob/main/contact.md#slack))

---

## 🎥 Videos & Blogs

- 📺 [OpenShift Developer Experience: Pipelines-as-Code](https://www.youtube.com/watch?v=PhqzGsJnFEI)  
- 📘 [How to make a release pipeline with Pipelines-as-Code](https://blog.chmouel.com/2021/07/01/how-to-make-a-release-pipeline-with-pipelines-as-code)

- 📝 **Latest Developer Documentation:** [Main branch docs](https://main.pipelines-as-code.pages.dev/)
