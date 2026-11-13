# NATSRadar Enterprise Architecture & Use Cases

This directory contains production architecture patterns, deployment blueprints, and reference guides for building resilient, cloud-native distributed messaging systems observed by **NATSRadar**.

---

## 📚 Use Cases & Architecture Blueprints

| Blueprint | Description | Key Capabilities |
|---|---|---|
| [**Cloud-to-Edge Hybrid Architecture with NATS Leaf Nodes**](./leaf-node-cloud-edge-architecture.md) | Connect private on-premises edge networks (retail stores, factory IoT, branch offices) to a central cloud hub. | Zero-inbound firewall traversal, 100% offline survivability, sub-millisecond local latency, and live RTT latency observability in NATSRadar. |

---

## 🚀 Quick Reference
- **NATS Leaf Nodes**: Extend central clusters to edge locations via outbound-only TLS (`port: 7422`).
- **Observability**: Monitor connected leaf nodes, bridged accounts, and message flow in NATSRadar's **`📡 Core NATS, Clients & Leaf Nodes`** panel.
