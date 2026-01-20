
<b>Pattern 1: When introducing new behavior branches (e.g., feature flags, unsupported event types, or ambiguous inputs), return early with a clear outcome (nil event / no-op) instead of bubbling up errors that create noisy or misleading provider statuses.
</b>

Example code before:
```
if isUnsupported(payload) {
  return nil, fmt.Errorf("unsupported event")
}
// later: provider sees error and posts "failed" status
```

Example code after:
```
if isUnsupported(payload) {
  logger.Infof("skipping unsupported event: %s", reason)
  return nil, nil // explicit no-op; caller treats as "ignore"
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2321#discussion_r2526171554
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2338#discussion_r2570635760
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2044#discussion_r2048435591
</details>


___

<b>Pattern 2: Guard against nil/absent nested fields in webhook payloads and templating contexts by using nil-safe getters where available and adding explicit checks only where direct field access can still panic (e.g., slices/maps without Getters).
</b>

Example code before:
```
// may panic if PullRequest is nil or Labels is nil
for _, l := range evt.PullRequest.Labels {
  labels = append(labels, l.Name)
}
```

Example code after:
```
pr := evt.GetPullRequest()
if pr != nil {
  for _, l := range pr.Labels { // still requires pr != nil
    labels = append(labels, l.GetName())
  }
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2348#discussion_r2610606932
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2348#discussion_r2610613693
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2264#discussion_r2394302093
</details>


___

<b>Pattern 3: Prefer structured/typed error handling for provider API failures; if only string matching is possible, constrain it to specific known non-actionable messages (e.g., status transition conflicts) and ensure behavior is applied consistently across similar code paths.
</b>

Example code before:
```
if err != nil {
  postMRComment("failed to set status: " + err.Error())
  return err
}
```

Example code after:
```
if err != nil {
  if strings.Contains(err.Error(), "Cannot transition status") {
    logger.Debugf("status already set; skipping MR comment: %v", err)
    return nil
  }
  logger.Warnf("failed to set status: %v", err)
  postMRComment("failed to set status; check token permissions")
  return err
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2587664872
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2680946184
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2698324996
</details>


___

<b>Pattern 4: In tests, avoid relying on deprecated external surfaces (e.g., repository status fields) and prefer stable signals (annotations/logs/checkruns), and when behavior changes from erroring to logging, update tests to assert on emitted logs instead of returned errors.
</b>

Example code before:
```
_, err := waitForRepoStatus("success")
assert.NilError(t, err)
```

Example code after:
```
logs := zapObserver.FilterMessageSnippet("cannot patch pipelinerun").TakeAll()
assert.Assert(t, len(logs) > 0)
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2363#discussion_r2639176544
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2338#discussion_r2570635760
</details>


___

<b>Pattern 5: Centralize commonly repeated logic (fallbacks, caching, helpers) behind a single accessor/helper method to avoid duplicated behavior across call sites (e.g., console URL fallback, changed-files caching, shared provider event parsing).
</b>

Example code before:
```
func detailURL(host string, ns, name string) string {
  if host == "" { host = "fallback" }
  return fmt.Sprintf("https://%s/...", host)
}
func namespaceURL(host string, ns string) string {
  if host == "" { host = "fallback" }
  return fmt.Sprintf("https://%s/...", host)
}
```

Example code after:
```
func (o *Console) Host() string {
  if o.host == "" { return "fallback" }
  return o.host
}
func (o *Console) DetailURL(ns, name string) string {
  return fmt.Sprintf("https://%s/...", o.Host())
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2288#discussion_r2424435108
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2317#discussion_r2517263653
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2044#discussion_r2052211429
</details>


___

<b>Pattern 6: Keep PRs focused: when adding a feature, avoid bundling refactors or deprecation cleanups in the same change unless strictly necessary; move unrelated maintenance into a separate PR/commit for traceability.
</b>

Example code before:
```
// Feature: accept new param
// Also: large refactor + deprecation removal + formatting churn
```

Example code after:
```
// PR 1: accept new param + tests
// PR 2: refactor internals / remove deprecated paths
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2290#discussion_r2432282457
</details>


___

<b>Pattern 7: Use consistent naming, types, and lint-aligned patterns: prefer idiomatic local variable casing, avoid redundant variables (reuse err), and align integer sizes with updated SDKs (int→int64) to prevent subtle mismatches in provider clients and tests.
</b>

Example code before:
```
OrgAndRepo := fmt.Sprintf("%s/%s", org, repo)
_, _, err2 := client.Do()
if err2 != nil { return err2 }
```

Example code after:
```
orgAndRepo := fmt.Sprintf("%s/%s", org, repo)
_, _, err := client.Do()
if err != nil { return err }
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2317#discussion_r2547733683
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2609490729
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2353#discussion_r2610238087
</details>


___
