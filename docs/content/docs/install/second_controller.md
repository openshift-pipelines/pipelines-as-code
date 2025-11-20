---
title: Multiple GitHub Applications Support
---

# Multi-GitHub Application Configuration

{{< tech_preview "Multi-GitHub Apps Support" >}}

Pipelines-as-Code allows multiple GitHub applications to operate on the same
cluster, enabling integration with different GitHub instances (e.g., public
GitHub and GitHub Enterprise Server).

## Deployment Architecture

Each GitHub application requires:

1. Dedicated controller deployment
2. Associated Service resource
3. Network exposure via Ingress (Kubernetes) or Route (OpenShift) or [smee.io](https://smee.io) for webhook tunneling
4. Unique configuration through:
   - Secret containing GitHub App credentials (`private key`, `application_id`, `webhook_secret`)
   - ConfigMap for application-specific settings

## Controller Configuration Parameters

| Environment Variable         | Description                                        | Example                |
| ---------------------------- | -------------------------------------------------- | ---------------------- |
| `PAC_CONTROLLER_LABEL`       | Unique identifier for the controller instance      | `github-enterprise`    |
| `PAC_CONTROLLER_SECRET`      | Secret containing GitHub App credentials           | `gh-enterprise-secret` |
| `PAC_CONTROLLER_CONFIGMAP`   | ConfigMap with application settings                | `gh-enterprise-config` |

{{< hint info >}}
**Note:** While each GitHub application requires its own controller, only one
status reconciler ("watcher") component is needed cluster-wide.
{{< /hint >}}

## Deployment Automation Script

The `second-controller.py` script makes it easy to generate the deployment yaml:

**Location:** `./hack/second-controller.py` in the [Pipelines-as-Code repository](https://github.com/openshift-pipelines/pipelines-as-code)

### Basic Usage

```bash
python3 hack/second-controller.py <LABEL> | kubectl apply -f -
```

### Advanced Options

```text
Usage: second-controller.py [-h] [--configmap CONFIGMAP]
                            [--ingress-domain INGRESS_DOMAIN]
                            [--secret SECRET]
                            [--controller-image CONTROLLER_IMAGE]
                            [--gosmee-image GOSMEE_IMAGE]
                            [--smee-url SMEE_URL] [--namespace NAMESPACE]
                            [--openshift-route]
                            LABEL
```

#### Key Options

| Option                     | Description                                                                   |
| -------------------------- | ----------------------------------------------------------------------------- |
| `--configmap`              | ConfigMap name (default: `<LABEL>-configmap`)                                 |
| `--secret`                 | Secret name (default: `<LABEL>-secret`)                                       |
| `--ingress-domain`         | Create Ingress with specified domain (Kubernetes)                             |
| `--openshift-route`        | Create OpenShift Route instead of Ingress                                     |
| `--controller-image`       | Custom controller image (use `ko` for local builds)                           |
| `--smee-url`               | Deploy Gosmee sidecar for webhook tunneling                                   |
| `--namespace`              | Target namespace (default: `pipelines-as-code`)                               |

### Example Scenarios

- Basic Kubernetes Deployment

```bash
# Generate and apply configuration for GitHub Enterprise
python3 hack/second-controller.py ghe \
  --ingress-domain "ghe.example.com" \
  --namespace pipelines-as-code | kubectl apply -f -
```

- OpenShift Deployment with Custom Config

```bash
# Create configuration with custom secret and route
python3 hack/second-controller.py enterprise \
  --openshift-route \
  --secret my-custom-secret \
  --configmap enterprise-config | oc apply -f -
```

- Local Development with Ko

```bash
# Build and deploy controller image using ko
export KO_DOCKER_REPO=quay.io/your-username
ko apply -f <(
  python3 hack/second-controller.py dev \
  --controller-image=ko \
  --namespace pipelines-as-code
)
```

**4. Webhook Tunneling with [Smee.io](https://smee.io)**

The tunneling avoid using a ingress route that is not accessible from the internet.

```bash
# Deploy with webhook tunneling for local testing
python3 hack/second-controller.py test \
  --smee-url https://smee.io/your-channel | kubectl apply -f -
```

### Environment Variables

The script respects these environment variables for customization:

```text
PAC_CONTROLLER_LABEL      Controller identifier
PAC_CONTROLLER_TARGET_NS  Target namespace (default: pipelines-as-code)
PAC_CONTROLLER_SECRET     Secret name (default: <LABEL>-secret)
PAC_CONTROLLER_CONFIGMAP  ConfigMap name (default: <LABEL>-configmap)
PAC_CONTROLLER_SMEE_URL   Smee.io URL for webhook tunneling
PAC_CONTROLLER_IMAGE      Controller image (default: ghcr.io/openshift-pipelines/pipelines-as-code-controller:stable)
```
