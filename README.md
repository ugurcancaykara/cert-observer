# Cert-Observer

A Kubernetes Operator that watches Ingress and Secret resources, tracks TLS certificate expiry dates, and reports summarized data via HTTP POST.

## Overview

Cert-Observer is a lightweight Kubernetes operator that:
- Watches all Ingress resources in the cluster
- Tracks TLS Secret resources and parses x509 certificate expiry dates
- Maintains an in-memory cache of Ingress and certificate data
- Periodically sends JSON reports to a configured HTTP endpoint (every 30-60s)

This simulates a multi-cluster observer system for tracking domain, DNS, and certificate status.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Kubernetes Cluster (Kind/Minikube)                    │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  cert-observer-system namespace                   │ │
│  │                                                    │ │
│  │  ┌──────────────────────────────────────────┐    │ │
│  │  │  cert-observer-controller                │    │ │
│  │  │                                          │    │ │
│  │  │  - IngressController (watches)          │    │ │
│  │  │  - SecretController (watches)           │    │ │
│  │  │  - IngressCache (in-memory)             │    │ │
│  │  │  - HTTPReporter (30s interval)          │    │ │
│  │  └──────────────────────────────────────────┘    │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │  default namespace                                │ │
│  │                                                    │ │
│  │  ┌─────────────┐  ┌─────────────┐                │ │
│  │  │ Ingress     │  │ TLS Secrets │                │ │
│  │  │ - webapp    │  │ - webapp-tls│                │ │
│  │  │ - api       │  │ - api-tls   │                │ │
│  │  └─────────────┘  └─────────────┘                │ │
│  │                                                    │ │
│  │  ┌──────────────────────────────────────────┐    │ │
│  │  │  test-server (receives reports)          │    │ │
│  │  │  - HTTP server :8080/report              │    │ │
│  │  └──────────────────────────────────────────┘    │ │
│  └───────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Components

- **IngressController**: Watches Ingress resources, extracts hosts and TLS configuration, updates cache
- **SecretController**: Watches TLS Secrets, parses x509 certificates, extracts expiry dates, updates cache
- **IngressCache**: Thread-safe in-memory cache (sync.RWMutex) storing transformed Ingress data
- **HTTPReporter**: Periodically reads from cache and sends JSON reports via HTTP POST

### Watch Mechanism

The operator uses controller-runtime's `Watches` feature to trigger Ingress reconciliation when TLS Secrets change:
- When a Secret is updated (e.g., certificate renewal), all Ingresses using that Secret are automatically reconciled
- This ensures certificate expiry dates are always up-to-date in reports

## Prerequisites

- Go 1.23+
- Docker
- kubectl
- Kind or Minikube
- Access to a Kubernetes cluster

## Quick Start

### 1. Create Kind Cluster

```bash
make kind-create
```

### 2. Build and Deploy

Build the operator image:
```bash
make docker-build IMG=cert-observer:latest
```

Load image to Kind:
```bash
make kind-load IMG=cert-observer:latest
```

Deploy to cluster:
```bash
make deploy IMG=cert-observer:latest
```

### 3. Deploy Test Server

Build and deploy the test server that receives reports:
```bash
cd examples/test-server
docker build -t test-server:latest .
kind load docker-image test-server:latest --name cert-observer
kubectl apply -f deployment.yaml
```

### 4. Apply Example Ingresses and Secrets

```bash
kubectl apply -f examples/webapp-secret.yaml
kubectl apply -f examples/webapp-ingress.yaml

kubectl apply -f examples/api-secret.yaml
kubectl apply -f examples/api-ingress.yaml

kubectl apply -f examples/blog-secret.yaml
kubectl apply -f examples/blog-ingress.yaml

kubectl apply -f examples/shop-secret.yaml
kubectl apply -f examples/shop-ingress.yaml

# Multi-host example
kubectl apply -f examples/multi-host-ingress.yaml
```

### 5. View Reports

Check the operator logs:
```bash
kubectl logs -n cert-observer-system deployment/cert-observer-controller-manager -f
```

Check test-server logs to see received reports:
```bash
kubectl logs test-server -f
```

## Configuration

The operator is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CLUSTER_NAME` | `local-cluster` | Cluster identifier in reports |
| `REPORT_ENDPOINT` | `http://test-server.default.svc.cluster.local:8080/report` | HTTP endpoint for reports |
| `REPORT_INTERVAL` | `30s` | Report sending interval |

To customize, edit `config/manager/manager.yaml`:

```yaml
env:
- name: CLUSTER_NAME
  value: "my-cluster"
- name: REPORT_ENDPOINT
  value: "http://collector.example.com/report"
- name: REPORT_INTERVAL
  value: "60s"
```

## Example JSON Output

```json
{
  "cluster": "local-cluster",
  "ingresses": [
    {
      "namespace": "default",
      "name": "webapp",
      "hosts": [
        {
          "host": "webapp.local",
          "certificate": {
            "name": "webapp-tls",
            "expires": "2026-11-20T10:30:00Z"
          }
        }
      ]
    },
    {
      "namespace": "default",
      "name": "multi-host",
      "hosts": [
        {
          "host": "test1.local",
          "certificate": {
            "name": "webapp-tls",
            "expires": "2026-11-20T10:30:00Z"
          }
        },
        {
          "host": "test2.local",
          "certificate": {
            "name": "webapp-tls",
            "expires": "2026-11-20T10:30:00Z"
          }
        },
        {
          "host": "test3.local",
          "certificate": {
            "name": "api-tls",
            "expires": "2026-11-20T10:31:00Z"
          }
        }
      ]
    }
  ]
}
```

## Testing

### Unit Tests

Run cache and configuration tests:
```bash
go test ./internal/cache/
go test ./internal/config/
```

### Integration Testing

1. Deploy operator and test-server
2. Apply example Ingresses
3. Verify reports in test-server logs:
   ```bash
   kubectl logs test-server --tail=100
   ```
4. Update a certificate and verify updated expiry in next report:
   ```bash
   # Generate new certificate
   openssl req -x509 -nodes -days 400 -newkey rsa:2048 \
     -keyout /tmp/new-key.pem -out /tmp/new-cert.pem \
     -subj "/CN=webapp.local"

   # Update secret
   kubectl delete secret webapp-tls
   kubectl create secret tls webapp-tls \
     --cert=/tmp/new-cert.pem --key=/tmp/new-key.pem

   # Check logs for updated expiry date
   kubectl logs test-server --tail=50
   ```

### Manual Testing Checklist

- [ ] Ingress create → appears in report
- [ ] Ingress update → report updated
- [ ] Ingress delete → removed from report
- [ ] Secret create → expiry date populated
- [ ] Secret update → new expiry in report
- [ ] Multi-host ingress → all hosts in report
- [ ] Ingress without TLS → no certificate info
- [ ] Reporter resilience (test-server down/up)

## Design Decisions

### In-Memory Cache

The operator uses a custom in-memory cache (`internal/cache/ingress_cache.go`) rather than relying solely on controller-runtime's built-in cache. This decision was made for:

1. **O(1) Read Performance**: Reporter reads from cache every 30s. With 1000 ingresses, transform-on-read would take ~100ms, while cache read takes ~1ms.

2. **Transform Once, Read Many**: Ingress data is transformed once (during reconciliation) and read multiple times (every report cycle). This is more efficient than transforming on every read.

3. **Certificate Expiry Preservation**: When Ingress reconciles (e.g., spec update), we preserve certificate expiry dates from cache, avoiding unnecessary Secret lookups.

4. **Task Requirement**: The home task explicitly requires "maintain a small in-memory cache."

**Trade-offs:**
- Memory: 2x storage (K8s cache + custom cache) - acceptable for small clusters
- Complexity: Manual cache sync in controllers - mitigated by watch-based updates
- Staleness: Cache is event-driven (only updates on resource changes) - acceptable for periodic reporting

### Event-Driven vs Polling

The operator uses Kubernetes watch events (via controller-runtime) rather than polling:
- Ingress/Secret changes trigger immediate reconciliation
- Cache updates only when resources actually change
- Reporter simply reads from cache every 30s (no K8s API calls)

This is more efficient than polling all resources every 30s.

## Development

### Build and Run Locally

```bash
# Run tests
make test

# Build
make build

# Run locally (against current kubeconfig context)
make run
```

### Generate Manifests

After modifying RBAC markers or CRDs:
```bash
make manifests
```

### Code Style

```bash
# Lint
make lint

# Format
go fmt ./...
```

## Cleanup

Delete the Kind cluster:
```bash
make kind-delete
```

## Troubleshooting

### Operator not starting

Check logs:
```bash
kubectl logs -n cert-observer-system deployment/cert-observer-controller-manager
```

Common issues:
- RBAC permissions: Verify `config/rbac/role.yaml` includes Ingress and Secret verbs
- Image not loaded: Ensure `make kind-load` succeeded
- Configuration errors: Check environment variables in deployment

### Reports not sending

1. Verify test-server is running:
   ```bash
   kubectl get pods -l app=test-server
   ```
2. Check operator logs for "report sent successfully"
3. Verify `REPORT_ENDPOINT` environment variable
4. Check network connectivity between namespaces

### Certificate expiry not showing

1. Verify Secret is type `kubernetes.io/tls`
2. Check Secret contains valid PEM-encoded certificate in `tls.crt`
3. Review SecretController logs for parsing errors

## Contributing

This project was built as a home task prototype. For production use, consider:
- Prometheus metrics endpoint
- ClusterObserver CRD for dynamic configuration
- Multi-cluster support
- Certificate expiry alerting
- Helm chart packaging

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
