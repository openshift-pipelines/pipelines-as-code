
<b>Pattern 1: When a feature needs extra operational visibility (CI/E2E workflows, controllers), explicitly add deterministic debug/diagnostic steps and wait for readiness (e.g., rollout status/wait) rather than relying on eventual config propagation or implicit restarts.
</b>

Example code before:
```
# Patch config and assume it will be picked up immediately
kubectl -n app patch configmap mycfg --type merge -p '{"data":{"loglevel":"debug"}}'
# ...continue without restart/wait...
```

Example code after:
```
set -euo pipefail
kubectl -n app patch configmap mycfg --type merge -p '{"data":{"loglevel":"debug"}}'
kubectl -n app rollout restart deployment/my-controller
kubectl -n app rollout status deployment/my-controller --timeout=120s
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2432#discussion_r2743633510
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2432#discussion_r2743637739
</details>


___

<b>Pattern 2: Avoid creating user-facing failures for non-critical follow-up operations (e.g., patching annotations/status updates) when the main action succeeded; instead, log the failure and add tests that assert the expected log/behavior rather than expecting an error return.
</b>

Example code before:
```
if err := patchResource(obj); err != nil {
  return nil, fmt.Errorf("cannot patch resource: %w", err)
}
return obj, nil
```

Example code after:
```
if err := patchResource(obj); err != nil {
  logger.Warnw("cannot patch resource", "err", err)
  // continue; main operation succeeded
}
return obj, nil
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2338#discussion_r2570635760
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2587664872
</details>


___

<b>Pattern 3: Prefer structured/typed error handling over broad fallbacks: classify provider/API errors (e.g., state transition vs permission) to decide whether to retry, skip secondary actions like MR comments, or surface actionable messages to users.
</b>

Example code before:
```
_, _, err := api.SetStatus(id, sha, opts)
if err != nil {
  // always post a comment on failure
  _ = api.CreateComment(id, "failed to set status")
}
```

Example code after:
```
_, _, err := api.SetStatus(id, sha, opts)
if err != nil {
  if errors.Is(err, ErrStatusAlreadySet) || strings.Contains(err.Error(), "Cannot transition status") {
    logger.Debugw("skipping comment; non-actionable status transition error", "err", err)
    return nil
  }
  logger.Warnw("failed to set status", "err", err)
  _ = api.CreateComment(id, "failed to set status; check token permissions")
}
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2587664872
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2680946184
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2341#discussion_r2684949199
</details>


___

<b>Pattern 4: In webhook parsing and templating, guard only the truly unsafe accesses: rely on SDK nil-safe getters where available, but add explicit nil checks for direct field/slice accesses (no getters) and handle nil/null payload bodies by treating them as empty objects.
</b>

Example code before:
```
// may panic if PullRequest is nil
for _, l := range evt.GetPullRequest().Labels {
  labels = append(labels, l.GetName())
}
// may fail if body is null
_ = json.Unmarshal(bodyBytes, &m)
```

Example code after:
```
pr := evt.GetPullRequest()
if pr != nil {
  for _, l := range pr.Labels { // direct slice; safe because pr != nil
    labels = append(labels, l.GetName())
  }
}
if len(bodyBytes) == 0 || string(bodyBytes) == "null" {
  bodyBytes = []byte(`{}`)
}
_ = json.Unmarshal(bodyBytes, &m)
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2348#discussion_r2610606932
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2348#discussion_r2610613693
</details>


___

<b>Pattern 5: Keep PRs tightly scoped: avoid mixing functional changes with refactors/deprecation cleanups in the same PR; if ancillary refactors are needed, split them into follow-up PRs/commits for clearer review and tracking.
</b>

Example code before:
```
# PR includes new feature + large refactor + deprecation cleanup in one change set
add_feature()
refactor_unrelated_modules()
remove_deprecated_paths()
```

Example code after:
```
# PR 1: add_feature()
# PR 2: refactor_unrelated_modules()
# PR 3: remove_deprecated_paths()
add_feature()
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2290#discussion_r2432282457
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2317#discussion_r2517263653
</details>


___

<b>Pattern 6: Centralize and reuse common logic (fallback values, error variables, naming conventions) to avoid duplication and inconsistencies: use getters/helpers for defaults, reuse existing variables instead of introducing err2, and follow consistent local variable casing.
</b>

Example code before:
```
func (c *Console) URL() string {
  if c.host == "" { return "https://fallback" }
  return "https://" + c.host
}
func (c *Console) DetailURL(ns, name string) string {
  if c.host == "" { return "https://fallback/..." } // duplicated fallback
  return fmt.Sprintf("https://%s/...", c.host)
}
if _, err2 := doThing(); err2 == nil { ... }
OrgAndRepo := fmt.Sprintf("%s/%s", org, repo)
```

Example code after:
```
func (c *Console) Host() string {
  if c.host == "" { return "openshift.url.is.not.configured" }
  return c.host
}
func (c *Console) URL() string { return "https://" + c.Host() }
func (c *Console) DetailURL(ns, name string) string {
  return fmt.Sprintf("%s/k8s/ns/%s/.../%s", c.URL(), ns, name)
}
if _, err := doThing(); err == nil { ... }
orgAndRepo := fmt.Sprintf("%s/%s", org, repo)
```

<details><summary>Examples for relevant past discussions:</summary>

- https://github.com/openshift-pipelines/pipelines-as-code/pull/2288#discussion_r2424435108
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2340#discussion_r2609490729
- https://github.com/openshift-pipelines/pipelines-as-code/pull/2317#discussion_r2547733683
</details>


___
