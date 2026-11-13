# 🚀 Getting Started with NATSRadar

NATSRadar is packaged as a single self-contained binary containing both the Go API backend and the React SPA UI embedded inside. It requires **zero external runtime dependencies**.

---

## 🐳 Option 1: Docker Compose (Fastest & Recommended)

Run a complete stack with NATS JetStream and NATSRadar with seeded demo data in one command:

```bash
# 1. Clone the repository
git clone https://github.com/codewithnithish/NATSRadar.git
cd NATSRadar

# 2. Start NATS and NATSRadar in the background
docker compose -f docker/docker-compose.yml up --build -d

# 3. Seed demo streams (ORDERS, PAYMENTS), KV buckets, and Object files
docker compose -f docker/docker-compose.yml --profile seed run --rm seed
```

Open your browser at **`http://localhost:8080`** and log in:
- **Default Username:** `admin`
- **Default Password:** `admin@123`

---

## 🔌 Option 2: Connect to an Existing NATS Server

If you already have a NATS server running locally or on your network:

```bash
docker compose -f docker/standalone.yml up -d
```
*(By default, this points to `nats://host.docker.internal:4222`)*

Or run the official pre-built container directly:

```bash
docker run -d \
  --name natsradar \
  -p 8080:8080 \
  -e NATS_URL="nats://my-nats-host:4222" \
  -e NATSRADAR_AUTH_USER="admin" \
  -e NATSRADAR_AUTH_PASS="admin@123" \
  ghcr.io/codewithnithish/natsradar:latest
```

---

## ☸️ Option 3: Kubernetes via Helm

Install NATSRadar into your Kubernetes cluster in seconds:

```bash
# Install the Helm chart
helm install natsradar k8s/charts/natsradar \
  --namespace natsradar --create-namespace \
  --set natsradar.natsUrl="nats://nats.default.svc.cluster.local:4222"

# Port-forward to access locally
kubectl port-forward -n natsradar svc/natsradar 8080:8080
```
Open **`http://localhost:8080`**.

---

## 💻 Option 4: Run Locally as Standalone Binary

```bash
# Build the single standalone binary
make build

# Run the binary
./bin/natsradar
```
By default, NATSRadar connects to `nats://127.0.0.1:4222` and listens on port `:8080`.
