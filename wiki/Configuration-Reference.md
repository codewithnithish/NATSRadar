# ⚙️ Configuration Reference

NATSRadar is configured via standard environment variables. All primary variables use the `NATSRADAR_*` prefix while retaining 100% backward-compatible fallback to `NATSCOPE_*`.

---

## 📋 Environment Variables

| Variable | Default | Description |
|---|---|---|
| `NATSRADAR_ADDR` | `:8080` | HTTP listen address (fallback: `PORT` or `:8080`) |
| `NATS_URL` | `nats://127.0.0.1:4222` | Default NATS connection created on first boot |
| `NATSRADAR_DEFAULT_CONNECTION_NAME` | `default` | Display name of the initial connection |
| `NATSRADAR_READ_ONLY` | `false` | When `true`, disables mutations (publishing, stream/consumer creation or deletion) |
| `NATSRADAR_STORAGE_PATH` | `.data/connections.json` | Local filesystem path where connection profiles are persisted |
| `NATSRADAR_AUTH_ENABLED` | `true` | Enables Web UI and API authentication |
| `NATSRADAR_AUTH_USER` | `admin` | Administrative login username |
| `NATSRADAR_AUTH_PASS` | `admin@123` | Administrative login password |
| `NATSRADAR_LOG_LEVEL` | `info` | Server log level (`debug`, `info`, `warn`, `error`) |

---

## 🗄️ Multi-Storage Backends

NATSRadar supports flexible storage backends for persisting cluster profiles and audit logs across pod restarts or distributed replicas:

### Local Disk / HostPath (Default)
```bash
NATSRADAR_STORAGE_TYPE="local"
NATSRADAR_STORAGE_PATH="/data/connections.json"
```

### AWS S3 / MinIO / Ceph / Cloudflare R2
```bash
NATSRADAR_STORAGE_TYPE="s3"
NATSRADAR_S3_ENDPOINT="s3.us-east-1.amazonaws.com"
NATSRADAR_S3_BUCKET="my-natsradar-bucket"
NATSRADAR_S3_REGION="us-east-1"
NATSRADAR_S3_PREFIX="natsradar/"
NATSRADAR_S3_ACCESS_KEY="AKIA..."
NATSRADAR_S3_SECRET_KEY="secret..."
NATSRADAR_S3_USE_SSL="true"
NATSRADAR_S3_STORAGE_CLASS="STANDARD"  # GLACIER, INTELLIGENT_TIERING, etc.
NATSRADAR_S3_TTL_DAYS="30"              # Automatic object lifecycle in days (0 = disabled)
NATSRADAR_S3_MAX_BYTES="107374182400"  # Storage quota limit in bytes (e.g. 100GB)
```
