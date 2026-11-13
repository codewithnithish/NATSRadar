# 🌿 Leaf Nodes & Edge Observability

NATS Leaf Nodes allow you to bridge independent, private edge networks (retail stores, factory IoT floors, branch offices, oil rigs) into a central cloud cluster with **zero inbound firewall ports**.

---

## 🏗️ Architecture Overview

In a typical hybrid edge deployment:
- The **Central Hub Cluster** runs in the cloud (AWS/GCP/Azure/Kubernetes) and listens on `port: 7422` for leaf node connections.
- The **Edge Leaf Node** runs on-premises and establishes an **outbound-only TLS connection** to the cloud hub.
- Local edge clients connect to the edge leaf node with sub-millisecond local latency.
- Even if WAN connectivity drops, local operations continue uninterrupted with local JetStream persistence; once restored, the leaf node automatically flushes all queued events to the cloud hub.

```
┌─────────────────────────────────┐
│     Private Edge Network        │
│  (Retail Store / IoT Factory)   │
│                                 │
│  ┌───────────────────────────┐  │
│  │   Edge Microservices      │  │
│  └─────────────┬─────────────┘  │
│                │ LAN (<0.5ms)   │
│                ▼                │
│  ┌───────────────────────────┐  │      Outbound TLS Handshake
│  │  🌿 NATS Leaf Node Server │──┼──────────────────────────────┐
│  └───────────────────────────┘  │      (Target: cloud:7422)    │
└─────────────────────────────────┘                              │
                                                                 │
┌─────────────────────────────────────────────────────────────┐  │
│                 ☁️ Central Cloud Infrastructure              │  │
│                                                             │  │
│  ┌───────────────────────────┐   Telemetry  ┌────────────┐  │  │
│  │ 🖥️ NATSRadar Dashboard     │<────────────│    Hub     │<─┘
│  │    (RTT: 4.2ms, Leaf Stats│              │    NATS    │
│  └───────────────────────────┘              └────────────┘
└─────────────────────────────────────────────────────────────┘
```

---

## 🔍 Leaf Node Telemetry in NATSRadar

NATSRadar connects to the central hub and continuously queries NATS `$SYS.REQ.SERVER.*.LEAFZ` telemetry to surface real-time metrics:

1. **Active Leaf Node List**:
   - Remote edge server identifier and account domain.
   - Connection status (**Active** vs **Disconnected**).
2. **Live Network Latency (RTT)**:
   - Measures Round-Trip Time latency (e.g. `🟢 4.2ms`) to detect WAN network degradation before outages occur.
3. **Traffic Counters**:
   - Inbound / Outbound message rates and cumulative byte throughput.
4. **Account Scoping**:
   - View which multi-tenant accounts are bridged across the leaf connection.
5. **Cluster Overview Stat Card**:
   - High-level metric in the Server panel showing total active edge leaves connected to the hub.
