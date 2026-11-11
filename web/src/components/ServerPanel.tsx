import { useQuery } from "@tanstack/react-query";
import { api, type ConnectionInfo } from "../api";
import { formatBytes, formatNumber } from "../format";

interface Props {
  connection: ConnectionInfo;
}

export function ServerPanel({ connection }: Props) {
  const server = useQuery({
    queryKey: ["server", connection.id],
    queryFn: () => api.serverDetails(connection.id),
    refetchInterval: 5000,
  });

  if (server.isPending) {
    return (
      <section className="card">
        <p className="muted pad">Loading server configuration & metrics…</p>
      </section>
    );
  }

  if (server.isError) {
    return (
      <section className="card">
        <div className="banner bad pad">{(server.error as Error).message}</div>
      </section>
    );
  }

  const s = server.data;
  const memPct = s.jsMaxMemory && s.jsMaxMemory > 0 && s.jsMemoryUsed
    ? Math.round((s.jsMemoryUsed / s.jsMaxMemory) * 100)
    : 0;
  const storagePct = s.jsMaxStorage && s.jsMaxStorage > 0 && s.jsStorageUsed
    ? Math.round((s.jsStorageUsed / s.jsMaxStorage) * 100)
    : 0;

  return (
    <div style={{ display: "grid", gap: 16 }}>
      {/* Top Stat Cards */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 14 }}>
        <div className="card pad">
          <span className="small muted">Server Version</span>
          <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
            v{s.version || connection.serverVersion || "N/A"}
          </div>
          <span className="small muted">{s.goVersion}</span>
        </div>

        <div className="card pad">
          <span className="small muted">Server Uptime</span>
          <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
            {s.uptime || "Online"}
          </div>
          <span className="small muted">CPU: {s.cpu.toFixed(1)}% ({s.cores} cores)</span>
        </div>

        <div className="card pad">
          <span className="small muted">Process Memory</span>
          <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
            {formatBytes(s.memBytes)}
          </div>
          <span className="small muted">{s.connections} active connections</span>
        </div>

        <div className="card pad">
          <span className="small muted">Network Messages</span>
          <div style={{ fontSize: 20, fontWeight: 700, marginTop: 4 }}>
            {formatNumber(s.inMsgs + s.outMsgs)}
          </div>
          <span className="small muted">
            {formatNumber(s.inMsgs)} in / {formatNumber(s.outMsgs)} out
          </span>
        </div>
      </div>

      {/* JetStream Storage Limits & Configuration */}
      <section className="card">
        <div className="card-head">
          <h2>JetStream Storage & Limits</h2>
          <span className={`pill ${s.jetStreamEnabled ? "ok" : "warn"}`}>
            {s.jetStreamEnabled ? "ENABLED" : "DISABLED"}
          </span>
        </div>

        {s.jetStreamEnabled ? (
          <div style={{ padding: 16, display: "grid", gap: 16 }}>
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                <span className="small"><strong>File Storage Used</strong></span>
                <span className="small muted">
                  {formatBytes(s.jsStorageUsed ?? 0)} / {formatBytes(s.jsMaxStorage ?? 0)} ({storagePct}%)
                </span>
              </div>
              <div className="meter-bar">
                <div className="meter-fill" style={{ width: `${Math.min(storagePct, 100)}%` }} />
              </div>
            </div>

            <div>
              <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                <span className="small"><strong>Memory Storage Used</strong></span>
                <span className="small muted">
                  {formatBytes(s.jsMemoryUsed ?? 0)} / {formatBytes(s.jsMaxMemory ?? 0)} ({memPct}%)
                </span>
              </div>
              <div className="meter-bar">
                <div className="meter-fill" style={{ width: `${Math.min(memPct, 100)}%` }} />
              </div>
            </div>

            {s.jsStoreDir && (
              <div className="small muted">
                Storage Directory: <code>{s.jsStoreDir}</code>
              </div>
            )}
          </div>
        ) : (
          <p className="muted pad">JetStream is not enabled on this server.</p>
        )}
      </section>

      {/* Cluster & Server Details */}
      <section className="card">
        <div className="card-head">
          <h2>Server Configuration Details</h2>
        </div>
        <div className="table-wrap">
          <table>
            <tbody>
              <tr>
                <td style={{ width: 220 }}><strong>Server Name</strong></td>
                <td><code>{s.serverName || s.serverId}</code></td>
              </tr>
              <tr>
                <td><strong>Server ID</strong></td>
                <td><code>{s.serverId}</code></td>
              </tr>
              <tr>
                <td><strong>Connected Endpoint</strong></td>
                <td><code>{connection.connectedUrl || connection.url}</code></td>
              </tr>
              {connection.hasTls && (
                <tr>
                  <td><strong>TLS / Security</strong></td>
                  <td>
                    <span className="pill ok">TLS Active</span>{" "}
                    {connection.insecureSkipVerify && (
                      <span className="pill warn">Skip Verify</span>
                    )}
                  </td>
                </tr>
              )}
              {s.clusterName && (
                <tr>
                  <td><strong>Cluster Name</strong></td>
                  <td><code className="chip">{s.clusterName}</code></td>
                </tr>
              )}
              <tr>
                <td><strong>Max Payload Size</strong></td>
                <td>{formatBytes(s.maxPayload)}</td>
              </tr>
              <tr>
                <td><strong>Total Client Connections</strong></td>
                <td>{formatNumber(s.totalConnections)} historical</td>
              </tr>
              <tr>
                <td><strong>Active Subscriptions</strong></td>
                <td>{formatNumber(s.subscriptions)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

