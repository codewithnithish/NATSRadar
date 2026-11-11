import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type ConnectionInfo } from "../api";

interface Props {
  connections: ConnectionInfo[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

function statusClass(status: string): string {
  switch (status) {
    case "CONNECTED":
      return "ok";
    case "RECONNECTING":
    case "CONNECTING":
      return "warn";
    default:
      return "bad";
  }
}

export function ConnectionBar({ connections, selectedId, onSelect }: Props) {
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [showTls, setShowTls] = useState(false);
  const [name, setName] = useState("");
  const [url, setUrl] = useState("nats://127.0.0.1:4222");
  const [readOnly, setReadOnly] = useState(false);
  const [insecureSkipVerify, setInsecureSkipVerify] = useState(false);
  const [serverName, setServerName] = useState("");
  const [caCert, setCaCert] = useState("");
  const [clientCert, setClientCert] = useState("");
  const [clientKey, setClientKey] = useState("");

  const add = useMutation({
    mutationFn: api.addConnection,
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({ queryKey: ["connections"] });
      onSelect(created.id);
      setAdding(false);
      setName("");
      setShowTls(false);
      setCaCert("");
      setClientCert("");
      setClientKey("");
      setServerName("");
      setInsecureSkipVerify(false);
    },
  });

  const remove = useMutation({
    mutationFn: api.removeConnection,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connections"] }),
  });

  const selected = connections.find((c) => c.id === selectedId);

  return (
    <div className="connbar">
      {connections.length > 0 && (
        <label className="field-inline">
          <span className="sr-only">Connection</span>
          <select value={selectedId ?? ""} onChange={(e) => onSelect(e.target.value)}>
            {connections.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} {c.hasTls ? "🔒" : ""} {c.readOnly ? "(RO)" : ""}
              </option>
            ))}
          </select>
        </label>
      )}

      {selected && (
        <>
          <span className={`pill ${statusClass(selected.status)}`}>{selected.status}</span>
          {selected.hasTls && <span className="pill ok" title="TLS Active">TLS</span>}
          {selected.readOnly && <span className="pill warn">READ-ONLY</span>}
          {selected.serverVersion && (
            <span className="muted small">nats-server {selected.serverVersion}</span>
          )}
          <button
            className="link danger"
            onClick={() => {
              if (window.confirm(`Remove connection "${selected.name}"? (This does not touch NATS.)`)) {
                remove.mutate(selected.id);
              }
            }}
          >
            Remove
          </button>
        </>
      )}

      <button className="btn" onClick={() => setAdding((v) => !v)}>
        {adding ? "Cancel" : "Add connection"}
      </button>

      {adding && (
        <form
          className="add-form"
          style={{ width: "100%", marginTop: 8 }}
          onSubmit={(e) => {
            e.preventDefault();
            add.mutate({
              name: name || url,
              url,
              readOnly,
              insecureSkipVerify,
              serverName: serverName.trim() || undefined,
              caCert: caCert.trim() || undefined,
              clientCert: clientCert.trim() || undefined,
              clientKey: clientKey.trim() || undefined,
            });
          }}
        >
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            <input
              placeholder="Name (e.g. k8s-prod)"
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-label="Connection name"
            />
            <input
              placeholder="tls://host:4222 or nats://..."
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              aria-label="NATS URL"
              required
              style={{ width: 240 }}
            />
            <label className="small muted" style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={readOnly}
                onChange={(e) => setReadOnly(e.target.checked)}
              />
              Read-Only
            </label>
            <button
              type="button"
              className="btn small"
              onClick={() => setShowTls(!showTls)}
            >
              {showTls ? "Hide TLS Options" : "🔒 TLS / Ingress Options"}
            </button>
            <button className="btn primary" type="submit" disabled={add.isPending}>
              {add.isPending ? "Connecting…" : "Connect"}
            </button>
          </div>

          {showTls && (
            <div style={{ marginTop: 8, padding: 12, border: "1px solid var(--border)", borderRadius: 6, background: "var(--code-bg)", display: "grid", gap: 8, width: "100%" }}>
              <div style={{ display: "flex", gap: 16, alignItems: "center" }}>
                <label className="small muted" style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer" }}>
                  <input
                    type="checkbox"
                    checked={insecureSkipVerify}
                    onChange={(e) => setInsecureSkipVerify(e.target.checked)}
                  />
                  Skip TLS Verification (InsecureSkipVerify)
                </label>
                <input
                  placeholder="SNI / ServerName override (for Ingress)"
                  value={serverName}
                  onChange={(e) => setServerName(e.target.value)}
                  style={{ width: 260 }}
                />
              </div>

              <div>
                <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                  Root CA Certificate (PEM text)
                </label>
                <textarea
                  placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                  value={caCert}
                  onChange={(e) => setCaCert(e.target.value)}
                  rows={3}
                  style={{ width: "100%", fontFamily: "monospace", fontSize: 11 }}
                />
              </div>

              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
                <div>
                  <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                    Client Certificate (for mTLS)
                  </label>
                  <textarea
                    placeholder="-----BEGIN CERTIFICATE-----"
                    value={clientCert}
                    onChange={(e) => setClientCert(e.target.value)}
                    rows={2}
                    style={{ width: "100%", fontFamily: "monospace", fontSize: 11 }}
                  />
                </div>
                <div>
                  <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                    Client Private Key (for mTLS)
                  </label>
                  <textarea
                    placeholder="-----BEGIN PRIVATE KEY-----"
                    value={clientKey}
                    onChange={(e) => setClientKey(e.target.value)}
                    rows={2}
                    style={{ width: "100%", fontFamily: "monospace", fontSize: 11 }}
                  />
                </div>
              </div>
            </div>
          )}

          {add.isError && <div className="error small" style={{ marginTop: 4 }}>{(add.error as Error).message}</div>}
        </form>
      )}
    </div>
  );
}
