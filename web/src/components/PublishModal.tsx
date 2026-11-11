import { useState } from "react";
import { api, type ConnectionInfo, type RequestResult, ApiError } from "../api";
import { formatBytes } from "../format";

interface Props {
  connection: ConnectionInfo;
  initialSubject?: string;
  initialPayload?: string;
  initialHeaders?: Record<string, string>;
  onClose: () => void;
}

export function PublishModal({
  connection,
  initialSubject = "",
  initialPayload,
  initialHeaders,
  onClose,
}: Props) {
  const [mode, setMode] = useState<"publish" | "request">("publish");
  const [subject, setSubject] = useState(initialSubject);
  const [payload, setPayload] = useState(initialPayload || '{\n  "hello": "nats"\n}');
  const [headers, setHeaders] = useState<Array<{ key: string; value: string }>>(() => {
    if (initialHeaders) {
      return Object.entries(initialHeaders).map(([key, value]) => ({ key, value }));
    }
    return [];
  });
  const [timeoutMs, setTimeoutMs] = useState(2000);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [response, setResponse] = useState<RequestResult | null>(null);

  const addHeader = () => {
    setHeaders([...headers, { key: "", value: "" }]);
  };

  const removeHeader = (index: number) => {
    setHeaders(headers.filter((_, i) => i !== index));
  };

  const updateHeader = (index: number, field: "key" | "value", val: string) => {
    const updated = [...headers];
    updated[index][field] = val;
    setHeaders(updated);
  };

  const formatJson = () => {
    try {
      const parsed = JSON.parse(payload);
      setPayload(JSON.stringify(parsed, null, 2));
      setError(null);
    } catch (e) {
      setError("Cannot format: invalid JSON syntax");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!subject.trim()) {
      setError("Subject is required");
      return;
    }

    const headerMap: Record<string, string> = {};
    for (const h of headers) {
      if (h.key.trim()) {
        headerMap[h.key.trim()] = h.value;
      }
    }

    setLoading(true);
    setError(null);
    setSuccessMsg(null);
    setResponse(null);

    try {
      if (mode === "publish") {
        const res = await api.publish(connection.id, {
          subject: subject.trim(),
          data: payload,
          headers: Object.keys(headerMap).length > 0 ? headerMap : undefined,
        });
        setSuccessMsg(`Published successfully (${res.bytes} bytes)`);
      } else {
        const res = await api.request(connection.id, {
          subject: subject.trim(),
          data: payload,
          headers: Object.keys(headerMap).length > 0 ? headerMap : undefined,
          timeoutMs,
        });
        setResponse(res);
      }
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`[${err.code}] ${err.message}`);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>Message Publisher & Tester</h2>
          <div className="mode-toggle">
            <button
              type="button"
              className={`mode-btn ${mode === "publish" ? "active" : ""}`}
              onClick={() => { setMode("publish"); setError(null); }}
            >
              Publish
            </button>
            <button
              type="button"
              className={`mode-btn ${mode === "request" ? "active" : ""}`}
              onClick={() => { setMode("request"); setError(null); }}
            >
              Request / Reply
            </button>
          </div>
        </div>
        <button className="link" onClick={onClose}>
          Close
        </button>
      </div>

      {connection.readOnly && (
        <div className="banner warn" style={{ margin: "12px 16px" }}>
          ⚠️ This connection is marked as <strong>Read-Only</strong>. Publishing and request operations are blocked by the server.
        </div>
      )}

      <form onSubmit={handleSubmit} style={{ padding: "16px", display: "grid", gap: "14px" }}>
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "10px", alignItems: "end" }}>
          <div>
            <label className="small muted" htmlFor="pub-subject" style={{ display: "block", marginBottom: 4 }}>
              Subject
            </label>
            <input
              id="pub-subject"
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="e.g. orders.create or dm.created"
              style={{ width: "100%" }}
              disabled={connection.readOnly}
              required
            />
          </div>

          {mode === "request" && (
            <div>
              <label className="small muted" htmlFor="pub-timeout" style={{ display: "block", marginBottom: 4 }}>
                Timeout (ms)
              </label>
              <select
                id="pub-timeout"
                value={timeoutMs}
                onChange={(e) => setTimeoutMs(Number(e.target.value))}
                disabled={connection.readOnly}
              >
                <option value={500}>500 ms</option>
                <option value={1000}>1000 ms (1s)</option>
                <option value={2000}>2000 ms (2s)</option>
                <option value={5000}>5000 ms (5s)</option>
              </select>
            </div>
          )}
        </div>

        <div>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
            <label className="small muted">Payload</label>
            <button type="button" className="link small" onClick={formatJson} disabled={connection.readOnly}>
              Format JSON
            </button>
          </div>
          <textarea
            value={payload}
            onChange={(e) => setPayload(e.target.value)}
            rows={6}
            style={{
              width: "100%",
              fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
              fontSize: "12.5px",
              padding: "10px",
              borderRadius: "6px",
              border: "1px solid var(--border)",
              background: "var(--surface)",
              color: "var(--text)",
            }}
            disabled={connection.readOnly}
          />
        </div>

        <div>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
            <label className="small muted">Headers (optional)</label>
            <button type="button" className="link small" onClick={addHeader} disabled={connection.readOnly}>
              + Add Header
            </button>
          </div>
          {headers.length > 0 && (
            <div style={{ display: "grid", gap: 6 }}>
              {headers.map((h, i) => (
                <div key={i} style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="text"
                    placeholder="Key"
                    value={h.key}
                    onChange={(e) => updateHeader(i, "key", e.target.value)}
                    style={{ flex: 1 }}
                    disabled={connection.readOnly}
                  />
                  <input
                    type="text"
                    placeholder="Value"
                    value={h.value}
                    onChange={(e) => updateHeader(i, "value", e.target.value)}
                    style={{ flex: 1 }}
                    disabled={connection.readOnly}
                  />
                  <button
                    type="button"
                    className="link danger small"
                    onClick={() => removeHeader(i)}
                    disabled={connection.readOnly}
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <button
            type="submit"
            className="btn primary"
            disabled={loading || connection.readOnly}
          >
            {loading ? "Sending…" : mode === "publish" ? "Publish Message" : "Send Request"}
          </button>
        </div>
      </form>

      {error && <div className="banner bad" style={{ margin: "0 16px 16px" }}>{error}</div>}
      {successMsg && <div className="banner ok" style={{ margin: "0 16px 16px" }}>{successMsg}</div>}

      {response && (
        <div style={{ margin: "0 16px 16px", padding: "14px", border: "1px solid var(--border)", borderRadius: "8px", background: "var(--surface)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
            <strong>Reply Received</strong>
            <span className="pill ok">{response.roundTripMs} ms</span>
            <span className="muted small">{formatBytes(response.size)}</span>
          </div>

          {response.headers && (
            <div className="msg-headers" style={{ marginBottom: 10 }}>
              <strong>Response Headers</strong>
              <table className="kv-table">
                <tbody>
                  {Object.entries(response.headers).map(([k, v]) => (
                    <tr key={k}>
                      <td><code>{k}</code></td>
                      <td>{v}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div>
            <strong className="small muted" style={{ display: "block", marginBottom: 4 }}>
              Response Payload{response.isBinary && " (base64)"}
            </strong>
            <pre className="msg-payload">{tryPretty(response.data)}</pre>
          </div>
        </div>
      )}
    </section>
  );
}

function tryPretty(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

