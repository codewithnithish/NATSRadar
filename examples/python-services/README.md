# Python Microservices with NATS & Natscope

This folder demonstrates testing all primary NATS communication patterns between two services:

1. **Core NATS (Pub/Sub)**: `notifications.alerts`
2. **Request / Reply**: `calculator.discount`
3. **JetStream Persistence**: `ORDERS` stream on `orders.created`

---

## Prerequisites

1. Ensure NATS is running (either locally or via Docker Compose):
   ```bash
   docker compose -f deploy/compose/docker-compose.yml up -d
   ```
2. Install dependencies:
   ```bash
   cd examples/python-services
   pip install -r requirements.txt
   ```

---

## Running the Services

### Terminal 1: Start Service 2 (Receiver / Worker)
```bash
python3 service_receiver.py
```
*Output: Listens on Core NATS, answers Request/Reply calculations, and fetches JetStream orders.*

### Terminal 2: Run Service 1 (Sender / Orchestrator)
```bash
python3 service_sender.py
```
*Output: Fires all three patterns, receives the reply, and exits.*

---

## Verifying in Natscope Web UI (http://localhost:8080)

1. **Core NATS & Clients tab**:
   - See active connected clients (`Service-1` and `Service-2`).
2. **Request / Reply tab**:
   - Inspect recent request-response messages.
3. **JetStream tab**:
   - View the `ORDERS` stream, message count, and storage size.
   - Click **Browse** on `ORDERS` to inspect the order JSON payloads.
   - Click the **Consumers** count on `ORDERS` to see consumer `orders-worker` ACK count and lag.

