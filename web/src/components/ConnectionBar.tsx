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

function authBadge(authType?: string) {
  switch (authType) {
    case "user_password":
      return <span className="pill ok" title="Username & Password Authentication">👤 User/Pass</span>;
    case "token":
      return <span className="pill ok" title="Bearer Token Authentication">🪙 Token</span>;
    case "nkey":
      return <span className="pill ok" title="NKey Seed Authentication">🔐 NKey</span>;
    case "credentials":
      return <span className="pill ok" title="Decentralized User Credentials (.creds)">📜 Creds</span>;
    case "tls_cert":
      return <span className="pill ok" title="mTLS Client Certificate Authentication">🪪 mTLS</span>;
    default:
      return null;
  }
}

export function ConnectionBar({ connections, selectedId, onSelect }: Props) {
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [showTls, setShowTls] = useState(false);
  const [showAuth, setShowAuth] = useState(false);
  const [authMode, setAuthMode] = useState<"none" | "user_pass" | "token" | "nkey" | "creds">("none");

  // Form states
  const [name, setName] = useState("");
  const [url, setUrl] = useState("nats://127.0.0.1:4222");
  const [readOnly, setReadOnly] = useState(false);

  // Auth fields
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [token, setToken] = useState("");
  const [nkey, setNKey] = useState("");
  const [credentials, setCredentials] = useState("");

  // TLS fields
  const [insecureSkipVerify, setInsecureSkipVerify] = useState(false);
  const [serverName, setServerName] = useState("");
  const [caCert, setCaCert] = useState("");
  const [clientCert, setClientCert] = useState("");
  const [clientKey, setClientKey] = useState("");

  const handleCredsFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (evt) => {
      const text = evt.target?.result as string;
      if (text) {
        setCredentials(text);
        if (!name) {
          setName(file.name.replace(/\.creds$/, ""));
        }
      }
    };
    reader.readAsText(file);
  };

  const add = useMutation({
    mutationFn: api.addConnection,
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({ queryKey: ["connections"] });
      onSelect(created.id);
      setAdding(false);
      setName("");
      setShowTls(false);
      setShowAuth(false);
      setAuthMode("none");
      setUser("");
      setPassword("");
      setToken("");
      setNKey("");
      setCredentials("");
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
                {c.name} {c.hasTls ? "🔒" : ""} {c.authType && c.authType !== "none" ? "🔑" : ""} {c.readOnly ? "(RO)" : ""}
              </option>
            ))}
          </select>
        </label>
      )}

      {selected && (
        <>
          <span className={`pill ${statusClass(selected.status)}`}>{selected.status}</span>
          {selected.hasTls && <span className="pill ok" title="TLS Active">TLS</span>}
          {authBadge(selected.authType)}
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
              user: authMode === "user_pass" ? user.trim() || undefined : undefined,
              password: authMode === "user_pass" ? password || undefined : undefined,
              token: authMode === "token" ? token.trim() || undefined : undefined,
              nkey: authMode === "nkey" ? nkey.trim() || undefined : undefined,
              credentials: authMode === "creds" ? credentials.trim() || undefined : undefined,
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
              className={`btn small ${showAuth || authMode !== "none" ? "active" : ""}`}
              onClick={() => setShowAuth(!showAuth)}
            >
              {showAuth ? "Hide Auth" : `🔑 Auth${authMode !== "none" ? ` (${authMode})` : ""}`}
            </button>
            <button
              type="button"
              className={`btn small ${showTls ? "active" : ""}`}
              onClick={() => setShowTls(!showTls)}
            >
              {showTls ? "Hide TLS" : "🔒 TLS / Ingress"}
            </button>
            <button className="btn primary" type="submit" disabled={add.isPending}>
              {add.isPending ? "Connecting…" : "Connect"}
            </button>
          </div>

          {showAuth && (
            <div style={{ marginTop: 8, padding: 12, border: "1px solid var(--border)", borderRadius: 6, background: "var(--code-bg)", display: "grid", gap: 10, width: "100%" }}>
              <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
                <span className="small muted" style={{ fontWeight: 600, marginRight: 4 }}>Auth Method:</span>
                <button
                  type="button"
                  className={`btn small ${authMode === "none" ? "primary" : ""}`}
                  onClick={() => setAuthMode("none")}
                >
                  None (Anonymous)
                </button>
                <button
                  type="button"
                  className={`btn small ${authMode === "user_pass" ? "primary" : ""}`}
                  onClick={() => setAuthMode("user_pass")}
                >
                  👤 User / Password
                </button>
                <button
                  type="button"
                  className={`btn small ${authMode === "token" ? "primary" : ""}`}
                  onClick={() => setAuthMode("token")}
                >
                  🪙 Token
                </button>
                <button
                  type="button"
                  className={`btn small ${authMode === "nkey" ? "primary" : ""}`}
                  onClick={() => setAuthMode("nkey")}
                >
                  🔐 NKey Seed
                </button>
                <button
                  type="button"
                  className={`btn small ${authMode === "creds" ? "primary" : ""}`}
                  onClick={() => setAuthMode("creds")}
                >
                  📜 User Creds (.creds)
                </button>
              </div>

              {authMode === "user_pass" && (
                <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "center", paddingTop: 4 }}>
                  <div>
                    <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                      Username
                    </label>
                    <input
                      placeholder="Username (e.g. admin or app-user)"
                      value={user}
                      onChange={(e) => setUser(e.target.value)}
                      style={{ width: 220 }}
                      required
                    />
                  </div>
                  <div>
                    <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                      Password
                    </label>
                    <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                      <input
                        type={showPassword ? "text" : "password"}
                        placeholder="Password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        style={{ width: 220 }}
                      />
                      <button
                        type="button"
                        className="btn small"
                        onClick={() => setShowPassword(!showPassword)}
                        title={showPassword ? "Hide password" : "Show password"}
                      >
                        {showPassword ? "🙈" : "👁️"}
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {authMode === "token" && (
                <div style={{ paddingTop: 4 }}>
                  <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                    Authentication Token
                  </label>
                  <input
                    type="password"
                    placeholder="Enter secret token"
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    style={{ width: 340 }}
                    required
                  />
                </div>
              )}

              {authMode === "nkey" && (
                <div style={{ paddingTop: 4 }}>
                  <label className="small muted" style={{ display: "block", marginBottom: 2 }}>
                    NKey User Seed (or File Path)
                  </label>
                  <input
                    type="password"
                    placeholder="SUA... or -----BEGIN USER NKEY SEED-----"
                    value={nkey}
                    onChange={(e) => setNKey(e.target.value)}
                    style={{ width: "100%", fontFamily: "monospace", fontSize: 12 }}
                    required
                  />
                  <span className="muted small" style={{ fontSize: 11, display: "block", marginTop: 2 }}>
                    Private seed starting with <code>SU</code>, or decorated PEM string.
                  </span>
                </div>
              )}

              {authMode === "creds" && (
                <div style={{ paddingTop: 4 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
                    <label className="small muted">
                      NATS Decentralized User Credentials (.creds)
                    </label>
                    <label className="btn small" style={{ cursor: "pointer", display: "inline-flex", alignItems: "center", gap: 4 }}>
                      📁 Load .creds file
                      <input type="file" accept=".creds,.txt" onChange={handleCredsFile} style={{ display: "none" }} />
                    </label>
                  </div>
                  <textarea
                    placeholder="Paste .creds file content containing JWT and NKey Seed (or click 'Load .creds file' above)"
                    value={credentials}
                    onChange={(e) => setCredentials(e.target.value)}
                    rows={4}
                    style={{ width: "100%", fontFamily: "monospace", fontSize: 11 }}
                    required
                  />
                </div>
              )}
            </div>
          )}

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
