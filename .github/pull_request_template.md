# Changes <!-- 🎉🎉🎉 Thank you for the PR!!! 🎉🎉🎉 -->

<!-- Describe your changes here- ideally you can get that description straight from
your descriptive commit message(s)! -->

# Submitter Checklist

- [ ] 📝 Please ensure your commit message is clear and informative. For guidance on crafting effective commit messages, refer to the How to write a git commit message guide. We prefer the commit message to be included in the PR body itself rather than a link to an external website (ie: Jira ticket).

- [ ] ♽ Before submitting a PR, run make test lint to avoid unnecessary CI processing. For an even more efficient workflow, consider installing [pre-commit](https://pre-commit.com/) and running pre-commit install in the root of this repository.

- [ ] ✨ We use linters to maintain clean and consistent code. Please ensure you've run make lint before submitting a PR. Some linters offer a --fix mode, which can be executed with the command make fix-linters (ensure [markdownlint](https://github.com/DavidAnson/markdownlint) and [golangci-lint](https://github.com/golangci/golangci-lint) tools are installed first).

- [ ] 📖 If you're introducing a user-facing feature or changing existing behavior, please ensure it's properly documented.

- [ ] 🧪 While 100% coverage isn't a requirement, we encourage unit tests for any code changes where possible.

- [ ] 🎁 If feasible, please check if an end-to-end test can be added. See [README](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/test/README.md) for more details.

- [ ] 🔎 If there's any flakiness in the CI tests, don't necessarily ignore it. It's better to address the issue before merging, or provide a valid reason to bypass it if fixing isn't possible (e.g., token rate limitations).
