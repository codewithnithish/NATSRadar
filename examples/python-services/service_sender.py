"""
Service 1 (Sender / Orchestrator):
- Core NATS: Publishes an alert to 'notifications.alerts'
- Request / Reply: Sends a query to 'calculator.discount' and awaits response
- JetStream: Publishes durable events to 'orders.created'
"""

import asyncio
import json
import logging
import random
import time
import nats

logging.basicConfig(level=logging.INFO, format="%(asctime)s [SERVICE-1] %(levelname)s: %(message)s")
logger = logging.getLogger("Service1")

async def main():
    logger.info("Connecting to NATS at nats://localhost:4222...")
    nc = await nats.connect("nats://localhost:4222")
    js = nc.jetstream()

    print("\n" + "=" * 60)
    print("🚀 SERVICE 1: SENDING TEST MESSAGES TO SERVICE 2 & JETSTREAM")
    print("=" * 60 + "\n")

    # 1. Test Core NATS Pub/Sub
    alert_payload = {
        "event": "SYSTEM_WARNING",
        "service": "Service-1",
        "cpu_usage": round(random.uniform(40.0, 95.0), 1),
        "timestamp": time.time()
    }
    logger.info("1️⃣ Publishing to Core NATS: 'notifications.alerts'...")
    await nc.publish("notifications.alerts", json.dumps(alert_payload).encode())
    logger.info(f"   Sent payload: {alert_payload}\n")

    # 2. Test Request / Reply Pattern
    calc_req = {"price": 250.0, "discount_pct": 15}
    logger.info(f"2️⃣ Sending Request to 'calculator.discount': {calc_req}...")
    try:
        reply_msg = await nc.request("calculator.discount", json.dumps(calc_req).encode(), timeout=3.0)
        reply_data = json.loads(reply_msg.data.decode())
        logger.info(f"   Received Reply from Service 2: {reply_data}\n")
    except Exception as e:
        logger.error(f"   Request failed (is Service 2 running?): {e}\n")

    # 3. Test JetStream Stream Publish
    order_id = f"ORD-{random.randint(1000, 9999)}"
    order_event = {
        "order_id": order_id,
        "customer": "Alice",
        "amount": round(random.uniform(10.0, 500.0), 2),
        "items": ["MacBook Pro Stand", "USB-C Hub"],
        "status": "PLACED"
    }
    logger.info(f"3️⃣ Publishing to JetStream Stream subject: 'orders.created' (Order: {order_id})...")
    ack = await js.publish("orders.created", json.dumps(order_event).encode())
    logger.info(f"   JetStream Ack received! Stream={ack.stream}, Sequence={ack.seq}\n")

    print("=" * 60)
    print("✅ All messages dispatched! Check Natscope Web UI (http://localhost:8080)")
    print("=" * 60 + "\n")

    await nc.drain()

if __name__ == "__main__":
    asyncio.run(main())

