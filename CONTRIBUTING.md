# Contributing to Natscope

Thanks for helping. Natscope is a small project, so the process is light.

## Setup

You need Go (latest stable), Node.js (LTS), `nats-server` and the `nats` CLI. See the README for a copy-paste setup.

```bash
make nats      # terminal 1: local NATS with JetStream
make seed      # once: demo streams and consumers
make dev-api   # terminal 2: Go server on :8080
make dev-web   # terminal 3: UI on :5173 (hot reload)
```

## Before opening a pull request

```bash
make fmt
make vet
make test
```

## Guidelines

- Browsing must never change NATS state. Reading messages must not create durable consumers or move a consumer's position.
- Anything that modifies NATS (publish, purge, delete) must respect the connection's read-only flag in the backend, not only in the UI.
- Keep packages small and focused: `api` (HTTP), `conn` (connections), `jsx` (JetStream reads), `webui` (embedded UI).
- Add a test with each behaviour change. Tests use an in-process NATS server (`internal/testutil`).
