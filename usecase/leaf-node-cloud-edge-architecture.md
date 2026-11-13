# Cloud-to-Edge Hybrid Architecture with NATS Leaf Nodes

This document details the enterprise hybrid-cloud architecture pattern using **NATS Leaf Nodes**, connecting private on-premises edge networks (retail stores, factory floors, branch offices, IoT gateways) to a central cloud cluster (AWS / GCP / Azure / Kubernetes), observed and managed by **Natscope**.

---

## 🏛️ High-Level Architecture Diagram

```mermaid
flowchart TD
    subgraph CloudVPC["☁️ Central Cloud Infrastructure (AWS / GCP / Azure / Kubernetes)"]
        direction TB

        Natscope["🖥️ Natscope UI & Observability Console<br/>(Port: 8080)"]
        
        subgraph CentralNATS["Central NATS Cluster (JetStream Hub)"]
            HubNATS["NATS Core & JetStream Engine<br/>Client: 4222 | Monitoring: 8222 | Leaf Listener: 7422"]
            CloudStreams[("Global JetStream Streams<br/>• ORDERS<br/>• TELEMETRY<br/>• AUDIT_LOGS")]
            CloudKV[("Global KV Buckets<br/>• global-configs<br/>• feature-flags")]
        end

        subgraph CloudServices["Cloud Microservices & Data Plane"]
            BillingSvc["💳 Billing & Payment Microservices"]
            AnalyticsEngine["📊 Real-Time Analytics & ML Pipeline"]
            S3Storage[("🗄️ Cloud Storage / AWS S3<br/>Object Store Archive")]
        end

        Natscope -.->|"Query Metrics & Telemetry"| HubNATS
        CloudServices <-->|"Pub / Sub & Req-Reply"| HubNATS
        HubNATS <--> CloudStreams
        HubNATS <--> CloudKV
        CloudStreams -.->|"Tiered Backup"| S3Storage
    end

    subgraph InternetWAN["🌐 Public Internet / Secure WAN"]
        direction TB
        TLSConnection["🔒 Outbound-Only mTLS Leaf Connection<br/>(Port: 7422)"]
    end

    subgraph PrivateEdge["🏭 Private On-Premises Network (Edge / Factory / Retail Store)"]
        direction TB

        EdgeFirewall["🛡️ Private Edge Firewall & NAT Gateway<br/>(Strict Policy: NO Inbound Ports Allowed)"]

        subgraph EdgeLeafNode["Local Edge NATS Server (Leaf Node)"]
            LocalNATS["Edge NATS Daemon<br/>Client: 4222 | Leaf Remote: 7422"]
            LocalCache[("Local JetStream Buffer & KV<br/>• edge-orders<br/>• local-cache")]
        end

        subgraph EdgeDevices["Private Network Clients & Microservices"]
            IoTSensors["🤖 IoT Sensors & Factory Equipment"]
            POSTerminals["🛒 POS Terminals / Store Registers"]
            EdgeApps["📦 Local Store Microservices & Workers"]
        end

        EdgeDevices <-->|"Ultra-Low Latency LAN (<1ms)"| LocalNATS
        LocalNATS <--> LocalCache
        LocalNATS ==>|"Initiates Outbound Handshake"| EdgeFirewall
    end

    EdgeFirewall ==>|"Outbound Encrypted Traffic Only"| TLSConnection
    TLSConnection ==>|"Connects to Listener"| HubNATS
```

---

## ⚡ Why This Architecture Solves Private Network Edge Challenges

### 1. Zero Inbound Firewall Holes (NAT Traversal)
- In traditional messaging architectures (Kafka, RabbitMQ, MQTT), bridging private branch offices requires opening inbound firewall ports or configuring fragile site-to-site IPsec VPN tunnels.
- **NATS Leaf Nodes solve this natively**: The Leaf Node in the private network **initiates an outbound TLS connection** on port `7422` to the cloud hub. Because the connection is outbound-only, it traverses stateful NAT and enterprise firewalls transparently with **zero inbound ports exposed**.

### 2. Local Autonomy & 100% Offline Survivability
- If the WAN or public internet connection drops:
  - Local microservices, POS terminals, and IoT sensors **continue operating at zero latency (<1ms)** against the local Edge NATS server.
  - Messages and KV mutations are stored safely in the local JetStream buffer.
- When internet connectivity is restored:
  - The Leaf Node automatically reconnects to the cloud hub.
  - Queued messages, state revisions, and stream sequences seamlessly synchronize without data loss.

### 3. Subject Namespace Scoping & Multi-Tenancy
- Edge nodes can be scoped to specific multi-tenant accounts (e.g. `Account: store-104` or `Account: factory-berlin`).
- Central Cloud applications can broadcast configurations down to all edge nodes (e.g. `configs.broadcast`), while each edge node isolates its telemetry to `edge.store-104.>`.

---

## 🔄 Bidirectional Message & Synchronization Flow

```mermaid
sequenceDiagram
    autonumber
    participant Client as 🛒 Local Edge Microservice
    participant Leaf as 🌿 Private Edge Leaf Node
    participant Hub as ☁️ Central Cloud NATS Hub
    participant CloudApp as 💳 Cloud Microservice / DB
    participant Natscope as 🖥️ Natscope Console

    Note over Leaf,Hub: 1. Outbound TLS Handshake across Edge Firewall
    Leaf->>Hub: Connect out to nats://cloud-nats:7422 (mTLS / NKey Auth)
    Hub-->>Leaf: Connection Accepted (Account: $G, Bridged)
    Hub->>Natscope: Emits Leafnode Metric (RTT: 4ms, Name: edge-store-01)

    Note over Client,Leaf: 2. Local Edge Publishing (Zero Network Latency)
    Client->>Leaf: Publish order.created {id: 101, total: 45.00}
    Leaf-->>Client: Fast ACK (< 1ms LAN)

    Note over Leaf,Hub: 3. Transparent Subject Bridging
    Leaf->>Hub: Transparently syncs order.created to Central Hub
    Hub->>CloudApp: Delivers order.created to Cloud Consumers
    Hub->>Natscope: Telemetry counters increment (+1 in/out)

    Note over Leaf,Hub: 4. Offline Scenario (WAN Cable Cut)
    Hub-xLeaf: Internet Connection Lost
    Client->>Leaf: Publish order.created {id: 102} (Stored locally in JetStream)
    Leaf-->>Client: Local ACK (Store operations never blocked)
    Natscope->>Natscope: Displays Leaf Node Status: Disconnected

    Note over Leaf,Hub: 5. WAN Restored & Auto-Catchup
    Leaf->>Hub: Auto-Reconnects & flushes buffered stream backlog
    Hub->>CloudApp: Receives queued messages in sequence
    Natscope->>Natscope: Status turns GREEN (RTT: 4ms, 1 Active Leaf)
```

---

## 🛠️ Production Configuration Templates

### 1. Central Cloud Hub Configuration (`cloud-central.conf`)
Deploy this on your central Kubernetes cluster or Cloud VM:

```hocon
# Server Identification
server_name: "cloud-central-hub"
port: 4222
http_port: 8222

# JetStream Engine
jetstream {
  store_dir: "/data"
  max_mem: 4G
  max_file: 50G
}

# Leaf Node Ingress Listener (Accepts Outbound connections from private branches)
leafnodes {
  port: 7422

  # Production Security: Enforce TLS & Authorization
  tls {
    cert_file: "/etc/nats/certs/server-cert.pem"
    key_file:  "/etc/nats/certs/server-key.pem"
    ca_file:   "/etc/nats/certs/ca.pem"
    verify_and_map: true
  }
}
```

---

### 2. Private Edge Leaf Node Configuration (`private-edge-leaf.conf`)
Deploy this inside the branch office, factory floor, or local Docker host:

```hocon
# Edge Server Identification
server_name: "edge-branch-leaf-01"
port: 4222
http_port: 8222

# Local JetStream Caching for Offline Resilience
jetstream {
  store_dir: "/edge-data"
  max_mem: 512M
  max_file: 5G
}

# Outbound Connection to Central Cloud (Zero Inbound Firewall Ports)
leafnodes {
  remotes = [
    {
      url: "tls://central-nats.mycloud.com:7422"
      credentials: "/etc/nats/tokens/edge-leaf-01.creds"

      # Optional subject mapping (Route edge topics to unique cloud prefixes)
      # import = [{ service: { subject: "cloud.services.>" } }]
      # export = [{ stream: { subject: "edge.branch-01.>" } }]
    }
  ]
}
```

---

## 🔍 Observability in Natscope

Natscope gives cloud and DevOps teams full visibility into the state of private leaf nodes without needing direct access to the edge local network:

1. **🌿 Connected Leaf Nodes Dashboard (`📡 Core NATS & Clients`)**:
   - **Name & Edge Identifier**: View which branch or factory is currently connected.
   - **Round-Trip Time (RTT)**: Live network latency measurement (e.g. `🟢 4.2ms`) to detect WAN network degradation.
   - **Direction & Role**: Displays `Hub (Inbound Leaf)` or `Spoke`.
   - **Account Scoping**: Identifies which NATS multi-tenant account is bridged.
   - **Network Throughput**: Live In/Out message rate and byte counters.
2. **Cluster Health Overview (`⚙️ Server & Cluster Info`)**:
   - High-level metric card displaying total **Connected Leaf Nodes: X Active**.
3. **Internal vs Microservice Filter**:
   - Easily toggle between your business microservice connections and internal management clients.
