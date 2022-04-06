<!-- 🎉🎉🎉 Thank you for the PR!!! 🎉🎉🎉 -->

# Changes

<!-- Describe your changes here- ideally you can get that description straight from
your descriptive commit message(s)! -->

# Submitter Checklist

- [ ] ♽  Run `make test lint` before submitting a PR (ie: via [pre-push github hook](../hack/dev/prep-push-hook) no need to waste CPU cycle on CI
- [ ] 📖 If you are adding a user facing feature or make a change of the behavior, please make sure to document it
- [ ] 🧪 100% coverage is not a target but most of the time we would rather have a unit test if you make a code change.
- [ ] 🎁 If that's something that is possible to do please make sure to check if we can add a e2e test.
- [ ] 🔎 If there is a flakiness in the CI tests then make sure to get the flakyness fixed before merging or if that's not possible there is a good reason to bypass it.

_See [the developer guide](https://github.com/openshift-pipelines/pipelines-as-code/blob/main/docs/development.md) for a bit more details._
