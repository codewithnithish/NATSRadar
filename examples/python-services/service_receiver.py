"""
Service 2 (Receiver / Worker):
- Core NATS: Subscribes to 'notifications.alerts'
- Request / Reply: Handles 'calculator.discount' requests and returns responses
- JetStream: Pulls messages from the 'ORDERS' stream on 'orders.created' and ACKs them
- KV Store: Updates its health heartbeat in 'system-state'
"""

import asyncio
import json
import logging
import nats
from nats.js.api import StreamConfig, RetentionPolicy

logging.basicConfig(level=logging.INFO, format="%(asctime)s [SERVICE-2] %(levelname)s: %(message)s")
logger = logging.getLogger("Service2")

async def main():
    logger.info("Connecting to NATS at nats://localhost:4222...")
    nc = await nats.connect("nats://localhost:4222")
    js = nc.jetstream()

    # 1. Core NATS Pub/Sub Handler
    async def on_alert(msg):
        data = msg.data.decode()
        logger.info(f"⚡ [Core NATS] Received on '{msg.subject}': {data}")

    await nc.subscribe("notifications.alerts", cb=on_alert)
    logger.info("📡 Subscribed to Core NATS subject: notifications.alerts")

    # 2. Request / Reply Handler
    async def on_calculator(msg):
        try:
            req = json.loads(msg.data.decode())
            price = req.get("price", 0)
            discount = req.get("discount_pct", 0)
            final_price = price * (1 - discount / 100.0)
            
            response = {
                "original_price": price,
                "discount_pct": discount,
                "final_price": round(final_price, 2),
                "processed_by": "Service-2"
            }
            logger.info(f"🔄 [Req/Rep] Handled request: {req} -> Final: {final_price}")
            await msg.respond(json.dumps(response).encode())
        except Exception as e:
            logger.error(f"Error handling request: {e}")
            await msg.respond(json.dumps({"error": str(e)}).encode())

    await nc.subscribe("calculator.discount", cb=on_calculator)
    logger.info("🤝 Listening for Request/Reply on: calculator.discount")

    # 3. JetStream Stream Setup & Durable Consumer
    try:
        # Check if ORDERS stream exists, create if not
        try:
            await js.stream_info("ORDERS")
            logger.info("📦 JetStream 'ORDERS' stream found.")
        except Exception:
            logger.info("📦 Creating JetStream 'ORDERS' stream...")
            await js.add_stream(name="ORDERS", subjects=["orders.>"])

        # Pull subscriber for JetStream
        sub = await js.pull_subscribe("orders.created", "orders-worker", stream="ORDERS")
        logger.info("📥 JetStream pull consumer 'orders-worker' active on 'orders.created'.")

        async def jetstream_worker():
            while True:
                try:
                    msgs = await sub.fetch(batch=5, timeout=1.0)
                    for m in msgs:
                        payload = json.loads(m.data.decode())
                        logger.info(f"⚡ [JetStream Stream: ORDERS] Seq={m.metadata.sequence.stream} Processed Order: {payload}")
                        await m.ack()
                except nats.errors.TimeoutError:
                    await asyncio.sleep(0.5)
                except Exception as ex:
                    await asyncio.sleep(1.0)

        asyncio.create_task(jetstream_worker())
    except Exception as e:
        logger.warning(f"JetStream setup notice: {e}")

    logger.info("🚀 Service 2 is RUNNING. Waiting for messages from Service 1...")

    try:
        while True:
            await asyncio.sleep(1)
    except asyncio.CancelledError:
        pass
    finally:
        await nc.drain()
        logger.info("Service 2 stopped gracefully.")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nShutdown requested.")

