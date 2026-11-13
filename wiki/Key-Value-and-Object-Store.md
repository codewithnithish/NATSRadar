# 🗄️ Key-Value & Object Store

NATS JetStream includes native distributed storage abstractions built on top of streams: **Key-Value (KV)** and **Object Store**. NATSRadar provides dedicated visual managers for both.

---

## 🗄️ Key-Value (KV) Store Explorer

NATS KV is a distributed, low-latency key-value store with revision tracking and TTL support.

### Capabilities:
- **Bucket Overview**: View bucket name, description, key count, history depth, TTL, and storage size.
- **Bucket Creation & Deletion**: Create buckets with custom history depth (e.g. keep last 5 revisions) and expiration TTL.
- **Key Navigation**: Search and filter keys in real-time.
- **Formatted Editor**: View and edit values directly in the browser with syntax-highlighted formatted JSON previews.
- **📜 Revision History**: Inspect the revision timeline of any key, tracking values, timestamps, and revision numbers.
- **Safe Key Deletion / Purge**: Delete keys with soft-delete tombstones or purge revision histories.

---

## 📦 Object Store Explorer

NATS Object Store provides distributed, chunked file storage for large assets, models, and binaries (up to multiple gigabytes) over the NATS protocol.

### Capabilities:
- **Bucket Overview**: List object storage buckets, file counts, and aggregate storage consumed.
- **File Browser**: View filenames, descriptions, byte sizes, chunk counts, and SHA-256 cryptographic digests.
- **📤 Browser Upload**: Upload files of any type directly from your browser into NATS Object Store without CLI tools.
- **⬇️ Direct Download**: Download stored objects with one click.
- **Metadata Inspection**: View chunk size, content digests, and creation timestamps.
- **Deletion**: Clean up files and teardown obsolete buckets.
