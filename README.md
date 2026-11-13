# NATSRadar

A modern, high-performance web UI, developer console, and observability platform for [NATS](https://nats.io) and JetStream.

Think **"Kafka UI / Redis Insight, for NATS"**.

[![Release](https://img.shields.io/github/v/release/codewithnithish/NATSRadar?logo=github&color=blue)](https://github.com/codewithnithish/NATSRadar/releases)
[![CI](https://github.com/codewithnithish/NATSRadar/actions/workflows/ci.yml/badge.svg)](https://github.com/codewithnithish/NATSRadar/actions)
[![Security Scan](https://github.com/codewithnithish/NATSRadar/actions/workflows/security.yml/badge.svg)](https://github.com/codewithnithish/NATSRadar/security/code-scanning)
[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fcodewithnithish%2Fnatsradar-blue?logo=docker)](https://github.com/codewithnithish/NATSRadar/pkgs/container/natsradar)
[![Helm Chart](https://img.shields.io/badge/helm-chart-0F1689?logo=helm)](k8s/charts/natsradar)
[![Go Version](https://img.shields.io/github/go-mod/go-version/codewithnithish/NATSRadar?logo=go)](go.mod)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

---

## ⚡ Highlights & Key Capabilities

- **⚡ JetStream Management**:
  - Streams listing with consumer lag, pending acks, and redeliveries rollup.
  - Stream CRUD: Create streams, purge messages, and delete streams.
  - **Full Consumer Lifecycle**: Create consumers (Durable/Ephemeral, AckPolicy, DeliverPolicy, FilterSubject), Pause delivery, Resume, and Delete consumers.
- **🔍 Advanced Message Browser & Time-Travel Search**:
  - Non-destructive direct-get message browsing (never consumes or acknowledges).
  - Subject wildcard filtering (e.g. filter `orders.us` in `orders.>`).
  - Search payload strings and custom headers.
  - **Time-Travel**: Jump directly to a timestamp (e.g. `10m ago`, `1h ago`) via server-side binary search.
  - **📥 Export JSON**: Download filtered messages for offline inspection.
  - **↻ DLQ Re-drive**: One-click re-drive/re-publish dead letter or poison pill messages.
- **🗄️ Key-Value (KV) Store Explorer**:
  - Browse KV buckets, values count, history depth, TTL, and storage size.
  - Create and delete buckets.
  - Key listing, search, value editor (with formatted JSON preview).
  - **📜 Revision History**: Inspect revision timeline for any key.
- **📦 Object Store Explorer**:
  - Browse buckets, file sizes, and storage backend.
  - Browser-based file upload and direct file downloads.
  - Inspect SHA-256 digests and metadata.
- **📡 Core NATS & Microservice Inspector**:
  - Discover which microservice clients are connected and subscribed to each subject (via NATS `$SYS.REQ.SERVER.*.CONNZ`).
  - **Live Subject Tailing**: Real-time SSE stream tailer with pause/resume and search filter.
  - **👁️ Smart Client Filtering**: One-click toggle between application microservices and internal management clients.
- **🌿 Leaf Nodes & Edge Observability**:
  - Discover remote edge NATS servers bridged to the central cloud cluster via Leaf Node protocol (`port: 7422`).
  - Live Round-Trip Time (RTT latency), inbound/outbound roles, bridged accounts, and message throughput.
  - [**Read the Cloud-to-Edge Hybrid Architecture Guide**](usecase/leaf-node-cloud-edge-architecture.md).
- **✉️ Request / Reply Tester**:
  - Synchronous request/reply tester with round-trip latency metrics and response inspection.
- **⚙️ Server & Cluster Overview**:
  - NATS runtime metrics (`/varz`), CPU/Memory usage, JetStream memory & disk storage meters.
- **🔐 Enterprise Security & Full NATS Authentication**:
  - **Full-Spectrum NATS Auth**: Connect to secured NATS clusters using **Username & Password**, **Auth Token**, **NKey Seed** (`SU...`), or **Decentralized User Credentials** (`.creds` with one-click file upload & JWT decoding).
  - **TLS & mTLS**: Custom Root CA certs, Client Certificate/Key (mTLS), InsecureSkipVerify, and SNI hostname overrides for Kubernetes Ingress.
  - **Auth Badges**: Real-time visual indicator badges (`👤 User/Pass`, `🪙 Token`, `🔐 NKey`, `📜 Creds`, `🪪 mTLS`) on active cluster connections.
  - **Built-in Console Authentication**: Built-in login screen, session management, and HTTP Basic Auth.
  - **Persistent Connections**: Auto-saves added connections to disk (`.data/connections.json`) with `0600` permissions.
- **☀️/🌙 Theme Switcher**:
  - Instant Light/Dark mode toggle with `localStorage` persistence.

---

## 🚀 Quick Start

### Option A: Using Docker Compose (Recommended)

Run NATS + NATSRadar with demo data in one command:

```bash
# 1. Start NATS and NATSRadar
docker compose -f docker/docker-compose.yml up --build -d

# 2. In another terminal, seed demo streams, KV buckets, and files:
docker compose -f docker/docker-compose.yml --profile seed run --rm seed
```

Or connect NATSRadar standalone to an existing NATS server on your machine:
```bash
docker compose -f docker/standalone.yml up -d
```

Open **http://localhost:8080** and sign in:
- **Username**: `admin`
- **Password**: `admin@123`

---

### Option B: Deploy on Kubernetes via Helm

Deploy directly to your Kubernetes cluster:

```bash
helm install natsradar k8s/charts/natsradar \
  --namespace natsradar --create-namespace \
  --set natsradar.natsUrl="nats://nats.default.svc.cluster.local:4222"
```

Port forward to open the UI:
```bash
kubectl port-forward -n natsradar svc/natsradar 8080:8080
```

---

### Option C: Local Standalone Binary

```bash
# Build the single standalone binary (React UI is embedded inside Go binary)
make build

# Start NATSRadar
./bin/natsradar
```

---

### Option C: Developer Mode (Hot Reload)

```bash
# 1. Local NATS server with JetStream
make nats

# 2. Seed demo streams (ORDERS, PAYMENTS), KV stores, and Object stores
make seed

# 3. Backend API server
make dev-api

# 4. Vite Frontend with hot reload
make dev-web
```

Open **http://localhost:5173**.

---

## ⚙️ Configuration (Environment Variables)

| Variable | Default | Description |
|---|---|---|
| `NATSRADAR_ADDR` | `:8080` | HTTP listen address (fallback: `NATSCOPE_ADDR`) |
| `NATS_URL` | `nats://127.0.0.1:4222` | Default NATS connection created on boot |
| `NATSRADAR_DEFAULT_CONNECTION_NAME` | `default` | Display name of the default connection |
| `NATSRADAR_READ_ONLY` | `false` | When `true`, disables stream/consumer mutation and publishing |
| `NATSRADAR_STORAGE_PATH` | `.data/connections.json` | Path where added connections are persisted |
| `NATSRADAR_AUTH_ENABLED` | `true` | Enables Web UI and API authentication |
| `NATSRADAR_AUTH_USER` | `admin` | Administrative login username |
| `NATSRADAR_AUTH_PASS` | `admin@123` | Administrative login password |
| `NATSRADAR_LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |

---

## 🗺️ REST API Reference

All write operations enforce the connection's `readOnly` setting. When `NATSRADAR_AUTH_ENABLED=true`, endpoints accept either the `natsradar_session` cookie (or legacy `natscope_session`) or `Authorization: Basic <credentials>` / `Bearer <token>`.

### Authentication
- `GET /api/auth/me` — Check current authentication state
- `POST /api/auth/login` — Sign in (`{"username": "admin", "password": "..."}`)
- `POST /api/auth/logout` — Invalidate session

### Connections
- `GET /api/connections` — List connections with auth status badges
- `POST /api/connections` — Register cluster (supports Username/Password, Token, NKey, `.creds` JWT, and TLS/mTLS)
- `DELETE /api/connections/{id}` — Remove connection

### Streams & Consumers
- `GET /api/connections/{id}/streams` — List streams with consumer lag
- `POST /api/connections/{id}/streams` — Create stream
- `DELETE /api/connections/{id}/streams/{stream}` — Delete stream
- `POST /api/connections/{id}/streams/{stream}/purge` — Purge stream messages
- `GET /api/connections/{id}/streams/{stream}/consumers` — List consumers & metrics
- `POST /api/connections/{id}/streams/{stream}/consumers` — Create consumer
- `DELETE /api/connections/{id}/streams/{stream}/consumers/{consumer}` — Delete consumer
- `POST /api/connections/{id}/streams/{stream}/consumers/{consumer}/pause` — Pause delivery
- `POST /api/connections/{id}/streams/{stream}/consumers/{consumer}/resume` — Resume delivery

### Messages & Real-Time
- `GET /api/connections/{id}/streams/{stream}/messages` — Direct-get messages (`startSeq`, `startTime`, `subject`, `search`, `limit`)
- `GET /api/connections/{id}/tail` — Live SSE event stream (`subject`)
- `POST /api/connections/{id}/publish` — Publish message (`subject`, `data`, `headers`)
- `POST /api/connections/{id}/request` — Send synchronous request & await reply (`timeoutMs`)
- `GET /api/connections/{id}/clients` — Discover connected microservice subscribers (`subject`)
- `GET /api/connections/{id}/server` — NATS server stats and JetStream storage limits (`/varz`)

### Key-Value (KV) Store
- `GET /api/connections/{id}/kv` — List KV buckets
- `POST /api/connections/{id}/kv` — Create KV bucket (`bucket`, `history`, `ttlSeconds`)
- `DELETE /api/connections/{id}/kv/{bucket}` — Delete KV bucket
- `GET /api/connections/{id}/kv/{bucket}/keys` — List keys
- `GET /api/connections/{id}/kv/{bucket}/keys/{key}` — Get key value & revision
- `POST /api/connections/{id}/kv/{bucket}/keys/{key}` — Put / update key
- `DELETE /api/connections/{id}/kv/{bucket}/keys/{key}` — Delete key
- `GET /api/connections/{id}/kv/{bucket}/keys/{key}/history` — Key revision history

### Object Store
- `GET /api/connections/{id}/objects` — List Object Store buckets
- `POST /api/connections/{id}/objects` — Create Object Store bucket (configurable storage type, TTL, max bytes, replicas)
- `DELETE /api/connections/{id}/objects/{bucket}` — Delete bucket
- `GET /api/connections/{id}/objects/{bucket}/list` — List files in bucket
- `GET /api/connections/{id}/objects/{bucket}/get?name=...` — Download object
- `POST /api/connections/{id}/objects/{bucket}/put?name=...` — Upload object body
- `DELETE /api/connections/{id}/objects/{bucket}/delete?name=...` — Delete object

---

## 💾 Multi-Storage Backend Architecture

NATSRadar supports flexible storage driver backends for configuration persistence, audit logs, and object store exports:

| Storage Provider | Configuration | Typical Deployment Scenario |
|---|---|---|
| **Local Disk / hostPath** | `NATSRADAR_STORAGE_TYPE=local` (default) | Local development, single-node Docker containers |
| **AWS S3 / MinIO / Ceph** | `NATSRADAR_STORAGE_TYPE=s3` | Cloud-native multi-replica clusters, high durability |
| **Kubernetes PVC / PV** | Standard Volume Mounts (`/data`) | Kubernetes Helm deployments (StatefulSet/Deployment) |
| **NFS / Network Shares** | Volume Mounts mapped to POSIX path | Enterprise on-prem shared storage |

### S3 & Object Store Environment Variables:
```bash
NATSRADAR_STORAGE_TYPE=s3
NATSRADAR_S3_ENDPOINT="s3.us-east-1.amazonaws.com"
NATSRADAR_S3_BUCKET="my-natsradar-bucket"
NATSRADAR_S3_REGION="us-east-1"
NATSRADAR_S3_PREFIX="natsradar/"
NATSRADAR_S3_ACCESS_KEY="AKIA..."
NATSRADAR_S3_SECRET_KEY="secret..."
NATSRADAR_S3_STORAGE_CLASS="STANDARD"  # GLACIER, INTELLIGENT_TIERING, etc.
NATSRADAR_S3_TTL_DAYS=30                # Automatic object expiration
NATSRADAR_S3_MAX_BYTES=107374182400    # Storage quota limit (e.g. 100GB)
```

---

## 🏛️ Project Architecture

```
NATSRadar
├── cmd/
│   ├── natsradar/            # Server entrypoint (single self-contained binary)
│   └── natsradar-seed/       # Realistic demo data generator (Streams, Consumers, KV, Objects)
├── docker/
│   ├── Dockerfile            # Multi-arch distroless container image
│   ├── docker-compose.yml    # NATS + NATSRadar complete local stack
│   └── standalone.yml        # Connect to existing NATS server
├── k8s/
│   └── charts/natsradar/     # Production Helm chart (Deployment, Service, PVC, Ingress)
├── internal/
│   ├── api/                  # REST API, auth middleware, SSE flusher, static UI file server
│   ├── config/               # Environment variable configuration parser
│   ├── conn/                 # NATS connection manager with TLS/mTLS & disk persistence
│   ├── jsx/                  # JetStream service layer (streams, consumers, KV, object store)
│   ├── storage/              # Multi-backend driver engine (Local, S3/MinIO, PVC, NFS)
│   ├── testutil/             # In-process test harness for integration testing
│   └── webui/                # Embedded React SPA build
├── web/                      # Modern React + TypeScript + Vite SPA
├── usecase/                  # Enterprise architecture blueprints & diagrams
└── examples/
    └── python-services/      # Microservice client simulations for live telemetry testing
```

---

## 📄 License

Licensed under the [Apache License, Version 2.0](LICENSE).
