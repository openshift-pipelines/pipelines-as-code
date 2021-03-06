<!-- ๐๐๐ Thank you for the PR!!! ๐๐๐ -->

# Changes

<!-- Describe your changes here- ideally you can get that description straight from
your descriptive commit message(s)! -->

# Submitter Checklist

- [ ] โฝ  Run `make test lint` before submitting a PR (ie: with [pre-commit](https://pipelinesascode.com/dev/tools), no need to waste CPU cycle on CI
- [ ] ๐ If you are adding a user facing feature or make a change of the behavior, please verify that you have documented it
- [ ] ๐งช 100% coverage is not a target but most of the time we would rather have a unit test if you make a code change.
- [ ] ๐ If that's something that is possible to do please ensure to check if we can add a e2e test.
- [ ] ๐ If there is a flakiness in the CI tests then don't *necessary* ignore it, better get the flakyness fixed before merging or if that's not possible there is a good reason to bypass it. (token rate limitation may be a good reason to skip).
