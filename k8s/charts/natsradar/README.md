# NATSRadar Helm Chart

A production-grade Helm chart for deploying **NATSRadar** (Visual Management, Monitoring & Observability Platform for NATS and JetStream) on Kubernetes.

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
helm install natsradar k8s/charts/natsradar \
  --namespace natsradar --create-namespace \
  --set natsradar.natsUrl="nats://nats.nats.svc.cluster.local:4222"
```

### 2. Access NATSRadar
To access the UI via port-forward:

```bash
kubectl port-forward -n natsradar svc/natsradar 8080:8080
```
Open **`http://localhost:8080`** in your browser and sign in with:
- **Username:** `admin` (or customized via values)
- **Password:** `admin@123`

---

## Configuration Reference

The following table lists the most common configurable parameters of the NATSRadar chart and their default values:

| Parameter | Description | Default |
| :--- | :--- | :--- |
| `replicaCount` | Number of NATSRadar replicas | `1` |
| `image.repository` | NATSRadar container image repository | `ghcr.io/codewithnithish/natsradar` |
| `image.tag` | NATSRadar container image tag | `""` (defaults to chart `appVersion`) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `natsradar.natsUrl` | Target NATS server endpoint | `nats://nats:4222` |
| `natsradar.defaultConnectionName` | Label for default connection in UI | `Kubernetes NATS` |
| `natsradar.readOnly` | Disables destructive actions (purge/delete/publish) | `false` |
| `natsradar.logLevel` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `natsradar.auth.enabled` | Enable UI/API authentication | `true` |
| `natsradar.auth.user` | Administrator username | `admin` |
| `natsradar.auth.password` | Administrator password | `admin@123` |
| `natsradar.auth.existingSecret` | Load credentials from an existing Secret | `""` |
| `persistence.enabled` | Persist saved connections across pod restarts | `true` |
| `persistence.size` | PVC disk capacity | `1Gi` |
| `persistence.storageClass` | StorageClass to use (empty uses default) | `""` |
| `ingress.enabled` | Create an Ingress resource | `false` |
| `ingress.className` | Ingress controller class name (e.g. `nginx`) | `""` |
| `ingress.hosts[0].host` | Ingress hostname | `natsradar.local` |
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory`| Memory request | `64Mi` |
| `resources.limits.cpu` | CPU limit | `300m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

---

## Common Recipes

### Using an Existing Kubernetes Secret for Passwords
If you manage credentials via HashiCorp Vault, Sealed Secrets, or External Secrets Operator:

```yaml
natsradar:
  auth:
    enabled: true
    existingSecret: "my-natsradar-credentials"
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
    - host: "natsradar.mycompany.internal"
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    - secretName: "natsradar-tls-cert"
      hosts:
        - "natsradar.mycompany.internal"
```

### Connecting to a Multi-Node NATS Cluster with TLS
```yaml
natsradar:
  natsUrl: "tls://nats-0.nats.nats.svc:4222,tls://nats-1.nats.nats.svc:4222,tls://nats-2.nats.nats.svc:4222"
```

---

## Uninstalling

```bash
helm uninstall natsradar --namespace natsradar
```
*(To also delete the persistent volume claim, run: `kubectl delete pvc -n natsradar natsradar-data`)*

