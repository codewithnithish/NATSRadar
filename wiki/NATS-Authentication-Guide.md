# 🔐 NATS Authentication Guide

NATSRadar supports every authentication scheme offered by modern NATS clusters (both Core NATS and NATS 2.0 Decentralized Account Security).

When adding a connection in the top navigation bar, click the **`🔑 Auth`** button to choose your authentication method:

---

## 1. 👤 Username & Password

Used for basic authenticated NATS server setups.

- **Inputs**:
  - `Username`: Microservice or administrator username (e.g. `billing-service`)
  - `Password`: Password (includes a show/hide toggle)
- **Supported URL Formats**:
  You can also embed credentials directly in the URL:
  `nats://myuser:mypassword@cluster.example.com:4222`

---

## 2. 🪙 Bearer / Auth Token

Used when NATS is configured with static or dynamic authorization tokens.

- **Inputs**:
  - `Token`: The shared secret token.
- **Supported URL Formats**:
  `nats://s3cr3t-t0k3n@cluster.example.com:4222`

---

## 3. 🔐 NKey Seed (`SU...`)

NKey is an Ed25519-based cryptographic authentication scheme without shared secrets. The client proves identity by signing server nonce challenges with a private seed.

- **Inputs**:
  - `NKey Seed`: A private user seed starting with `SU` (e.g. `SUAH4732LKJD...`), or a decorated PEM string (`-----BEGIN USER NKEY SEED-----`).
- **Security**: The seed is never sent over the wire or read from server files; it signs challenges in-memory and wipes ephemeral memory immediately.

---

## 4. 📜 Decentralized User Credentials (`.creds`)

Standard authentication in NATS 2.0 multi-tenant account architectures (e.g. Synadia Cloud, NGS, or self-hosted operators).

A `.creds` file is a chained file containing:
1. `-----BEGIN NATS USER JWT-----`
2. `-----BEGIN USER NKEY SEED-----`

### How to Connect in NATSRadar:
1. Click **`📁 Load .creds file`** to select your `.creds` file from disk. The browser locally reads the file and fills in the credentials!
2. Or paste the content of your `.creds` file directly into the textarea.

---

## 5. 🪪 TLS Client Certificates (mTLS)

Used in zero-trust enterprise environments where mutual TLS authentication is required.

Click **`🔒 TLS / Ingress`** in the connection form:
- **Root CA Certificate**: Custom internal or enterprise Root CA in PEM format.
- **Client Certificate**: Public X.509 certificate in PEM format.
- **Client Private Key**: Private key corresponding to the client certificate.
- **SNI ServerName**: Override TLS server name for Kubernetes Ingress or routing controllers.
- **InsecureSkipVerify**: Check to bypass certificate validation for local self-signed setups.

---

## 🏷️ Visual Connection Status Badges

Once connected, your connection displays an auth indicator pill in the navigation header:

| Badge | Meaning |
|---|---|
| `👤 User/Pass` | Connected via Username and Password |
| `🪙 Token` | Connected via Bearer Token |
| `🔐 NKey` | Connected via NKey Cryptographic Seed |
| `📜 Creds` | Connected via NATS 2.0 User Credentials (`.creds`) |
| `🪪 mTLS` | Connected via Mutual TLS Client Certificate |
| `TLS` | TLS encryption active |
| `READ-ONLY` | Write/delete mutations blocked |
