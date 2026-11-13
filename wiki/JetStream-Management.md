# ⚡ JetStream Management & Observability

JetStream is NATS's built-in distributed persistence engine. NATSRadar provides complete visibility and operational control over streams, consumers, and message payloads.

---

## 1. 📊 Stream Operations

### Stream Listing & Metrics
- **Consumer Lag Rollup**: Instantly view unconsumed message counts across all attached consumers.
- **Pending ACKs & Redeliveries**: Detect slow or struggling consumers experiencing duplicate deliveries or delivery timeouts.
- **Storage Meters**: Visual storage utilization bar showing message count and bytes relative to stream limits.

### Stream Actions
- **Create Stream**: Configure stream name, subject bindings (with wildcards like `orders.>`), retention policy (`Limits`, `Interest`, `WorkQueue`), storage type (`File` vs `Memory`), and max age/bytes.
- **Purge Messages**: Flush messages from a stream without deleting the stream configuration or consumer bindings.
- **Delete Stream**: Safely teardown streams with user confirmation.

---

## 2. 👥 Consumer Lifecycle & Control

NATSRadar gives developers granular control over JetStream consumers:

| Action | Description |
|---|---|
| **Create Consumer** | Define Durable name, Filter Subject, Deliver Policy (`All`, `Last`, `New`, `ByStartTime`), Ack Policy (`Explicit`, `All`, `None`), Ack Wait, and Max Deliveries. |
| **⏸️ Pause Delivery** | Temporarily suspend message delivery to consumers during downstream maintenance or database outages without stopping the consumer. |
| **▶️ Resume Delivery** | Resume message delivery with zero data loss. |
| **Delete Consumer** | Clean up ephemeral or obsolete consumers. |

---

## 3. 🔍 Time-Travel Message Browser

Browse and search stream messages **non-destructively**:

- **Direct-Get Access**: NATSRadar fetches messages directly using JetStream sequence lookups; it **never acknowledges or alters consumer offsets**.
- **Subject Filtering**: Filter messages by exact subject or wildcards (e.g. `orders.us.created`).
- **Full-Text Payload & Header Search**: Filter stream history by string keywords or JSON keys.
- **Time-Travel Jump**: Select timestamps like `10m ago`, `1h ago`, `24h ago`, or custom dates. Server-side binary search locates the matching stream sequence in milliseconds.
- **📥 JSON Export**: Download filtered messages as a JSON file for offline inspection or incident analysis.
- **↻ DLQ Re-Drive**: Re-publish poison-pill or dead-letter messages to target subjects with one click.
