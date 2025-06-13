## 📝 Change Description

<!-- (this should be a clear and concise description of all changes in this PR) -->

## 🔗 Linked GitHub Issue

Closes #

## 👨🏻‍ Linked Jira

<!-- This is optional, but if you have a Jira ticket related to this PR, please link it here. -->
## 🚀 Type of Change

<!-- (update the title of the Pull Request accordingly) -->

- [ ] 🐛 Bug fix (`fix:`)
- [ ] ✨ New feature (`feat:`)
- [ ] 💥 Breaking change (`feat!:`, `fix!:`)
- [ ] 📚 Documentation update (`docs:`)
- [ ] ⚙️ Chore (`chore:`)
- [ ] 💅 Refactor (`refactor:`)

## 🧪 Testing Strategy

- [ ] Unit tests
- [ ] Integration tests
- [ ] End-to-end tests
- [ ] Manual testing
- [ ] Note Applicable

## ✅ Submitter Checklist

- [ ] 📝 My commit messages are clear, informative, and follow the project's [How to write a git commit message guide](https://developers.google.com/blockly/guides/contribute/get-started/commits). **The [Gitlint](https://jorisroovers.com/gitlint/latest) linter ensures in CI it's properly validated**
- [ ] ✨ I have ensured my commit message prefix (e.g., `fix:`, `feat:`) matches the "Type of Change" I selected above.
- [ ] ♽ I have run `make test` and `make lint` locally to check for and fix any
      issues. For an efficient workflow, I have considered installing
      [pre-commit](https://pre-commit.com/) and running `pre-commit install` to
      automate these checks.
- [ ] 📖 I have added or updated documentation for any user-facing changes.
- [ ] 🧪 I have added sufficient unit tests for my code changes.
- [ ] 🎁 I have added end-to-end tests where feasible. See [README](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/test/README.md) for more details.
- [ ] 🔎 I have addressed any CI test flakiness or provided a clear reason to bypass it.
- [ ] If adding a provider feature, I have filled in the following and updated the provider documentation:
  - [ ] GitHub App
  - [ ] GitHub Webhook
  - [ ] Gitea/Forgejo
  - [ ] GitLab
  - [ ] Bitbucket Cloud
  - [ ] Bitbucket Data Center
