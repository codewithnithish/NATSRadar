# ☸️ Kubernetes & Helm Deployment

NATSRadar includes a production-grade Helm chart located in `k8s/charts/natsradar`.

---

## 🚀 Quick Install

```bash
helm install natsradar k8s/charts/natsradar \
  --namespace natsradar --create-namespace \
  --set natsradar.natsUrl="nats://nats.nats.svc.cluster.local:4222"
```

Access the UI locally:
```bash
kubectl port-forward -n natsradar svc/natsradar 8080:8080
```
Open **`http://localhost:8080`**.

---

## 🔒 Security Best Practices

### 1. Existing Secrets for Admin Passwords
Instead of plain text values, reference an existing Kubernetes Secret managed by HashiCorp Vault, Sealed Secrets, or External Secrets Operator:

```yaml
natsradar:
  auth:
    enabled: true
    existingSecret: "my-natsradar-credentials"
    userKey: "USERNAME"
    passwordKey: "PASSWORD"
```

### 2. Ingress Controller with cert-manager TLS
Deploy NATSRadar with automatic Let's Encrypt TLS:

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

### 3. Persistent Volumes
To retain user-added connection profiles across pod restarts:

```yaml
persistence:
  enabled: true
  type: "pvc"
  pvc:
    storageClass: "gp3"
    size: 1Gi
```
