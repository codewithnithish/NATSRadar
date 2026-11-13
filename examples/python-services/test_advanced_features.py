"""
Advanced NATS Testing Script:
1. Key-Value Store (KV):
   - Creates bucket 'system-configs' (with revision history enabled)
   - Puts JSON configs and updates keys multiple times to create revisions (Rev 1, Rev 2, Rev 3)
   - Creates bucket 'user-sessions' with active user data

2. Object Store (OS):
   - Creates bucket 'reports'
   - Uploads 'performance_audit_2026.json'
   - Uploads 'architecture_spec.md' with SHA-256 digest calculation

Run this script, then inspect everything in Natscope UI!
"""

import asyncio
import json
import logging
import nats
from nats.js.api import KeyValueConfig, ObjectStoreConfig

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("AdvancedTest")

async def main():
    logger.info("Connecting to NATS at nats://localhost:4222...")
    nc = await nats.connect("nats://localhost:4222")
    js = nc.jetstream()

    print("\n" + "=" * 65)
    print("🗄️  1. TESTING KEY-VALUE (KV) STORE")
    print("=" * 65)

    # 1. Create KV bucket 'system-configs' with history=5 so we can view revision history
    try:
        kv = await js.create_key_value(bucket="system-configs", history=5)
        logger.info("✅ Created KV bucket: 'system-configs' (History retention: 5)")
    except Exception:
        kv = await js.key_value("system-configs")
        logger.info("ℹ️ KV bucket 'system-configs' already exists, using existing.")

    # Put initial config keys
    await kv.put("database.settings", json.dumps({
        "host": "postgres.internal",
        "port": 5432,
        "database": "production_db",
        "pool_size": 25,
        "ssl_mode": "require"
    }, indent=2).encode())
    logger.info("   -> Put 'database.settings' (JSON configuration)")

    await kv.put("auth.jwt_secret", b"s3cr3t-t0k3n-2026-natscope")
    logger.info("   -> Put 'auth.jwt_secret' (String value)")

    # Put multiple revisions of 'app.environment' to test revision history in UI!
    rev1 = await kv.put("app.environment", json.dumps({"env": "development", "debug": True, "version": "1.0.0"}).encode())
    logger.info(f"   -> Put 'app.environment' [Revision {rev1}]")
    await asyncio.sleep(0.5)

    rev2 = await kv.put("app.environment", json.dumps({"env": "staging", "debug": True, "version": "1.0.1"}).encode())
    logger.info(f"   -> Updated 'app.environment' [Revision {rev2}]")
    await asyncio.sleep(0.5)

    rev3 = await kv.put("app.environment", json.dumps({"env": "production", "debug": False, "version": "1.1.0"}).encode())
    logger.info(f"   -> Updated 'app.environment' [Revision {rev3}]")

    # 2. Create another bucket 'user-sessions'
    try:
        kv_users = await js.create_key_value(bucket="user-sessions", history=1)
        logger.info("✅ Created KV bucket: 'user-sessions'")
    except Exception:
        kv_users = await js.key_value("user-sessions")

    await kv_users.put("session.user_9821", json.dumps({
        "user_id": 9821,
        "username": "nithish",
        "role": "SuperAdmin",
        "authenticated_via": "SSO",
        "ip_address": "127.0.0.1"
    }).encode())
    logger.info("   -> Put 'session.user_9821' in 'user-sessions'")

    print("\n" + "=" * 65)
    print("📦  2. TESTING OBJECT STORE")
    print("=" * 65)

    # 1. Create Object Store bucket 'reports'
    try:
        os = await js.create_object_store(bucket="reports")
        logger.info("✅ Created Object Store bucket: 'reports'")
    except Exception:
        os = await js.object_store("reports")
        logger.info("ℹ️ Object Store bucket 'reports' already exists, using existing.")

    # 2. Upload JSON Report document
    report_content = json.dumps({
        "report_id": "RPT-2026-Q4",
        "generated_by": "Natscope Automation",
        "metrics": {
            "total_throughput_msg_sec": 48250,
            "average_latency_ms": 0.42,
            "jetstream_storage_efficiency": "99.4%"
        },
        "servers_evaluated": ["nats-1", "nats-2", "nats-3"]
    }, indent=2).encode()

    info1 = await os.put("performance_audit_2026.json", report_content)
    logger.info(f"   -> Uploaded 'performance_audit_2026.json' ({info1.size} bytes, digest: {info1.digest[:16]}...)")

    # 3. Upload a Markdown document
    markdown_doc = b"""# NATSRadar Architecture Overview
NATSRadar provides full-duplex introspection and management over:
- NATS Core Pub/Sub
- Request / Reply RPC
- JetStream Streams and Durable Consumers
- JetStream Key-Value (KV) Buckets & Revision Trees
- JetStream Object Stores & Chunked File Blocks
"""
    info2 = await os.put("architecture_spec.md", markdown_doc)
    logger.info(f"   -> Uploaded 'architecture_spec.md' ({info2.size} bytes, digest: {info2.digest[:16]}...)")

    print("\n" + "=" * 65)
    print("⚙️   3. SERVER & CLUSTER TELEMETRY")
    print("=" * 65)

    # Query internal $SYS for real-time memory and stats
    try:
        sub = await nc.request("$SYS.REQ.SERVER.PING.VARZ", b"", timeout=2.0)
        varz = json.loads(sub.data.decode()).get("data", {})
        logger.info(f"   Server Name:    {varz.get('server_name')}")
        logger.info(f"   Server Version: {varz.get('version')}")
        logger.info(f"   Active Conns:   {varz.get('connections')}")
        logger.info(f"   RAM In-Use:     {varz.get('mem')} bytes (~{round(varz.get('mem', 0) / 1024 / 1024, 2)} MB)")
        logger.info(f"   JetStream Mem:  {varz.get('js_mem_storage', 0)} bytes")
        logger.info(f"   JetStream Disk: {varz.get('js_disk_storage', 0)} bytes")
    except Exception as e:
        logger.warning(f"Could not query $SYS directly: {e}")

    print("\n" + "=" * 65)
    print("🎉 ALL DATA SEEDED SUCCESSFULLY!")
    print("Open NATSRadar UI (http://localhost:8080) to inspect:")
    print("   1. 🗄️  KV Store       -> Click 'system-configs' -> View keys and click 'History'")
    print("   2. 📦 Object Store    -> Click 'reports' -> View files, SHA-256 digests, and click '⬇ Download'")
    print("   3. ⚙️  Server Info    -> View live RAM, JetStream disk usage, and server telemetry")
    print("=" * 65 + "\n")

    await nc.drain()

if __name__ == "__main__":
    asyncio.run(main())

