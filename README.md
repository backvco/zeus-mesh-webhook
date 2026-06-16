# zeus-mesh-webhook

Mutating admission webhook for [Zeus](https://github.com/backvco/zeus) that automatically injects Zeus Mesh CA trust into every pod on mesh-enrolled clusters.

## What it does

When a cluster is enrolled in the Zeus mesh, all cross-cluster TLS traffic is signed by the Zeus Mesh CA. This webhook ensures every pod trusts that CA automatically — no per-service configuration needed.

On each pod CREATE, the webhook injects:

| What | Where |
|------|-------|
| Volume: `zeus-mesh-ca-bundle` ConfigMap | `/etc/zeus-mesh-certs/` |
| `SSL_CERT_DIR` | `/etc/ssl/certs:/etc/zeus-mesh-certs` |
| `NODE_EXTRA_CA_CERTS` | `/etc/zeus-mesh-certs/ca.crt` |

This covers Go, Python, Ruby, and Node.js runtimes. Java uses its own keystore and is a known exception.

## Design

- **`failurePolicy: Ignore`** — webhook downtime never blocks pod startup
- **Excluded by default**: `kube-system`, `kube-public`, `kube-node-lease`, `cert-manager`, `zeus-overlay`, `ingress-nginx`
- **TLS**: cert issued by the `zeus-mesh-ca` cert-manager ClusterIssuer (present on all enrolled clusters)
- **Idempotent**: skips injection if env vars or volume mount already exist

## Installation

This webhook is installed automatically by Zeus during mesh enrollment. It is not intended to be installed manually.

## Image

```
ghcr.io/backvco/zeus-mesh-webhook:latest
```

Multi-arch: `linux/amd64`, `linux/arm64`.

Images are signed with [cosign](https://github.com/sigstore/cosign) keyless signing via Sigstore. Verify with:

```bash
cosign verify ghcr.io/backvco/zeus-mesh-webhook:latest \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp "^https://github.com/backvco/zeus-mesh-webhook/.github/workflows/release.yml"
```

## Building

```bash
make build          # host platform binary
make release        # linux/amd64 + linux/arm64 binaries
make docker         # multi-arch image push to ghcr.io
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--tls-cert` | `/tls/tls.crt` | TLS certificate file |
| `--tls-key` | `/tls/tls.key` | TLS private key file |
| `--addr` | `:8443` | Listen address |
| `--ca-configmap` | `zeus-mesh-ca-bundle` | ConfigMap name with `ca.crt` |
| `--ca-dir` | `/etc/zeus-mesh-certs` | Mount path in injected pods |

Additional namespaces can be excluded via the `EXCLUDED_NAMESPACES` environment variable (comma-separated).
