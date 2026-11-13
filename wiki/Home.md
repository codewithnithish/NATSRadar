# ◈ Welcome to the NATSRadar Wiki

**NATSRadar** is a modern, high-performance web UI, developer console, and real-time observability platform for [NATS](https://nats.io) and JetStream.

Think **"Kafka UI / Redis Insight, but engineered specifically for NATS"**.

---

## 🚀 Key Modules at a Glance

| Module | What You Can Do |
| :--- | :--- |
| **⚡ JetStream Engine** | Create, inspect, purge, and delete streams. Monitor consumer lag, pending acks, pause/resume delivery, and re-drive DLQ messages. |
| **🔍 Time-Travel Browser** | Non-destructive direct-get message inspection. Search by payload text, headers, sequence numbers, or jump to timestamps (*e.g., `10m ago`*). |
| **🗄️ Key-Value (KV) Store** | Explore buckets, view key revision history trees, edit values live with JSON formatting, and delete keys/buckets. |
| **📦 Object Store** | Browse files, upload documents/binaries directly from the browser, inspect SHA-256 chunk digests, and download files. |
| **📡 Core NATS Inspector** | Discover active microservice connections, view subscribed subjects, and live-tail messages in real-time with SSE. |
| **🌿 Leaf Nodes & Edge** | Monitor private edge servers bridged into your cloud hub via outbound TLS (`port 7422`) with live Round-Trip Time (RTT) latency. |
| **🔐 Full NATS Auth Suite** | Connect seamlessly using **Username/Password**, **Bearer Tokens**, **NKey Seeds**, **Decentralized `.creds`**, or **mTLS certificates**. |

---

## 📚 Quick Links & Documentation Map

- **[Getting Started](Getting-Started)**: Run NATSRadar in 60 seconds with Docker Compose, Helm, or standalone binary.
- **[NATS Authentication Guide](NATS-Authentication-Guide)**: How to configure Username/Password, Token, NKey, and `.creds` authentication.
- **[JetStream Deep Dive](JetStream-Management)**: Streams, Consumers, Delivery policies, and message search.
- **[Key-Value & Object Stores](Key-Value-and-Object-Store)**: KV revision trees and chunked object management.
- **[Leaf Nodes & Edge Observability](Leaf-Nodes-and-Edge-Observability)**: Connect and observe retail, factory, or branch edge clusters.
- **[Configuration Reference](Configuration-Reference)**: All `NATSRADAR_*` environment variables and storage configurations.
- **[Kubernetes & Helm Deployment](Kubernetes-and-Helm-Deployment)**: Production Helm setup, Ingress with TLS, and Secret management.

---

## 🏛️ System Architecture

```
                    ┌───────────────────────────┐
                    │  Web Browser / Operations │
                    └─────────────┬─────────────┘
                                  │ HTTP / SSE (:8080)
                                  ▼
                    ┌───────────────────────────┐
                    │         NATSRadar         │
                    │   (Go + Embedded React)   │
                    └─────────────┬─────────────┘
                                  │ NATS Protocol (4222 / 7422)
          ┌───────────────────────┼───────────────────────┐
          ▼                       ▼                       ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│  Core NATS Hub   │    │ JetStream Engine │    │ Edge Leaf Nodes  │
│  (Pub/Sub, RPC)  │    │ (Streams, KV, OS)│    │ (Retail/IoT Edge)│
└──────────────────┘    └──────────────────┘    └──────────────────┘
```
