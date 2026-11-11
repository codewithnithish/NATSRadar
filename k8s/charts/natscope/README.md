# Natscope Helm Chart

A production-grade Helm chart for deploying **Natscope** (Visual Management, Monitoring & Observability Platform for NATS and JetStream) on Kubernetes.

---

## Features

- ⚡ **Auto-Discovery:** Automatically connects to your Kubernetes in-cluster NATS (`nats://nats.default.svc:4222`) or any external NATS URL.
- 🔒 **Security-First:** Runs as non-root user (`65532`), drops all Linux capabilities, and supports existing Kubernetes Secrets for auth passwords.
- 💾 **Stateful Persistence:** Retains added connection profiles across pod restarts via a PersistentVolumeClaim.
- 🌐 **Ingress Ready:** Pre-configured for Traefik, NGINX Ingress Controller, and cert-manager TLS certificates.
- 🩺 **Kubernetes Probes:** Native HTTP liveness & readiness health checks (`/api/health`).

---

## Quickstart

### 1. Install the Chart
From the root of the repository:

```bash
helm install natscope deploy/helm/natscope \
  --namespace natscope --create-namespace \
  --set natscope.natsUrl="nats://nats.nats.svc.cluster.local:4222"
```

### 2. Access Natscope
To access the UI via port-forward:

```bash
kubectl port-forward -n natscope svc/natscope 8080:8080
```
Open **`http://localhost:8080`** in your browser and sign in with:
- **Username:** `admin` (or customized via values)
- **Password:** `admin@123`

---

## Configuration Reference

The following table lists the most common configurable parameters of the Natscope chart and their default values:

| Parameter | Description | Default |
| :--- | :--- | :--- |
| `replicaCount` | Number of Natscope replicas | `1` |
| `image.repository` | Natscope container image repository | `codewithnithish/natscope` |
| `image.tag` | Natscope container image tag | `""` (defaults to chart `appVersion`) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `natscope.natsUrl` | Target NATS server endpoint | `nats://nats:4222` |
| `natscope.defaultConnectionName` | Label for default connection in UI | `Kubernetes NATS` |
| `natscope.readOnly` | Disables destructive actions (purge/delete/publish) | `false` |
| `natscope.logLevel` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `natscope.auth.enabled` | Enable UI/API authentication | `true` |
| `natscope.auth.user` | Administrator username | `admin` |
| `natscope.auth.password` | Administrator password | `admin@123` |
| `natscope.auth.existingSecret` | Load credentials from an existing Secret | `""` |
| `persistence.enabled` | Persist saved connections across pod restarts | `true` |
| `persistence.size` | PVC disk capacity | `1Gi` |
| `persistence.storageClass` | StorageClass to use (empty uses default) | `""` |
| `ingress.enabled` | Create an Ingress resource | `false` |
| `ingress.className` | Ingress controller class name (e.g. `nginx`) | `""` |
| `ingress.hosts[0].host` | Ingress hostname | `natscope.local` |
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory`| Memory request | `64Mi` |
| `resources.limits.cpu` | CPU limit | `300m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

---

## Common Recipes

### Using an Existing Kubernetes Secret for Passwords
If you manage credentials via HashiCorp Vault, Sealed Secrets, or External Secrets Operator:

```yaml
natscope:
  auth:
    enabled: true
    existingSecret: "my-natscope-credentials"
    userKey: "USERNAME"
    passwordKey: "PASSWORD"
```

### Enabling Ingress with TLS
```yaml
ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: "natscope.mycompany.internal"
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    - secretName: "natscope-tls-cert"
      hosts:
        - "natscope.mycompany.internal"
```

### Connecting to a Multi-Node NATS Cluster with TLS
```yaml
natscope:
  natsUrl: "tls://nats-0.nats.nats.svc:4222,tls://nats-1.nats.nats.svc:4222,tls://nats-2.nats.nats.svc:4222"
```

---

## Uninstalling

```bash
helm uninstall natscope --namespace natscope
```
*(To also delete the persistent volume claim, run: `kubectl delete pvc -n natscope natscope-data`)*

