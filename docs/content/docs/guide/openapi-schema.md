---
title: OpenAPI Schema Validation
weight: 80
---

# OpenAPI Schema Validation for Repository CRDs

Pipelines-as-Code provides [OpenAPI](https://www.openapis.org/) schema validation for its Custom Resource
Definitions (CRDs), which helps in writing the Repository resources. This page
explains what OpenAPI schemas are, their benefits, and how to leverage them in
your development environment.

## What is OpenAPI Schema Validation?

OpenAPI schema validation is a mechanism that adds metadata to Kubernetes CRDs, providing:

- **Type information**: Specifies expected data types for fields
- **Required fields**: Marks which fields must be present
- **Pattern validation**: Enforces specific formats (e.g., URLs must start with http:// or https://)
- **Enumeration validation**: Limits fields to a predefined set of values
- **Field descriptions**: Documents the purpose of each field

When you create or modify a Repository resource, the OpenAPI schema helps
validate your configuration before it's applied to the cluster, catching errors
early in the development process.

## Benefits of OpenAPI Schema Validation

Using OpenAPI schemas with Pipelines-as-Code provides numerous advantages:

1. **Early Error Detection**: Identifies configuration errors before applying resources to your cluster
2. **Improved Documentation**: Field descriptions provide built-in documentation
3. **Better IDE Support**: Enables rich autocompletion and validation in code editors
4. **Standardized Formatting**: Ensures your resources follow expected formats
5. **Self-Documenting API**: Makes it easier to understand the Repository CRD structure

## Using OpenAPI Schemas in VS Code

Visual Studio Code and other modern editors can leverage OpenAPI schemas to
provide real-time validation, autocompletion, and documentation while editing
your Repository resources.

### Setting Up VS Code for CRD Validation

![VS Code and OpenAPI Schema](/images/vscode-openapi-schema.png)

1. Install the [Kubernetes](https://marketplace.visualstudio.com/items?itemName=ms-kubernetes-tools.vscode-kubernetes-tools) extension (by Microsoft)
2. Install the [YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) extension (by Red Hat)

These extensions will automatically detect and use the OpenAPI schema from your Kubernetes cluster when editing Repository resources.

### Example: Editing a Repository Resource in VS Code

When editing a Repository resource in VS Code with the appropriate extensions, you'll experience:

- **Autocompletion** for fields like `url`, `git_provider`, `settings`, etc.
- **Validation** for required fields and field formats
- **Documentation tooltips** when hovering over fields
- **Enum dropdown suggestions** for fields with predefined values

Here's an example of what you'll see:

```yaml
apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: my-repository
spec:
  # VS Code will show an error if this required field is missing
  url: https://github.com/myorg/myrepo

  # Autocompletion will suggest all available fields
  git_provider:
    # Dropdown will show available provider types
    type: github

    # Pattern validation ensures the URL format is correct
    url: https://api.github.com

    # Documentation tooltips show field descriptions
    webhook_secret:
      name: webhook-secret
      key: token

  # Validation for minimum values
  concurrency_limit: 5

  settings:
    # Enum validation limits to specific options
    pipelinerun_provenance: default_branch
```

## Other Tools Using OpenAPI Schemas

Beyond VS Code, OpenAPI schemas are useful with:

1. **kubectl**: Validates resources before applying them

   ```bash
   kubectl create -f my-repository.yaml --validate=true
   ```

2. **kube-linter**: Validates resources as part of CI/CD pipelines

3. **Kubernetes Dashboard**: Shows field descriptions and validations

4. **OpenAPI documentation generators**: Create API documentation from schemas

## How to Get the OpenAPI Schema

The OpenAPI schema for Repository resources is embedded in the CRD itself. You can view it with:

```bash
kubectl get crd repositories.pipelinesascode.tekton.dev -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema}' | jq
```

Or to see all available fields with their descriptions:

```bash
kubectl explain repository.spec --recursive
```

## Troubleshooting Schema Validation Errors

If you encounter validation errors when creating or updating a Repository:

1. **Check field types**: Ensure values match expected types
2. **Verify required fields**: Make sure all required fields are present
3. **Review pattern validations**: URLs must follow the correct format
4. **Check enum values**: Some fields accept only specific values

Error messages typically provide specific details about which validation failed, making it easier to correct your resources.

{{< hint info >}}
Even with client-side validation, the webhook validation in Pipelines-as-Code provides an additional layer of validation for complex logic that cannot be expressed in OpenAPI schemas.
{{< /hint >}}
