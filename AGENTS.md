Read all those files:

@.github/pull_request_template.md
@.cursor/rules/architecture.mdc
@.cursor/rules/code-formatting.mdc
@.cursor/rules/dependencies.mdc
@.cursor/rules/documentation.mdc
@.cursor/rules/e2e-testing.mdc
@.cursor/rules/git-commit-format.mdc
@.cursor/rules/jira-formatting.mdc
@.cursor/rules/pre-commit.mdc
@.cursor/rules/srvkp-jira-bug-template.mdc
@.cursor/rules/srvkp-jira-template.mdc
@.cursor/rules/testing-quality.mdc
@.cursor/rules/useful-commands.mdc

# Editing code

- always do a make fix-python-errors after editing python files
- always do a make fix-markdownlint editing markdown files
- always do a make fumpt after editing go files
- always do a make fix-trailings-spaces editing markdown files
