import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type LeafNodeInfo } from "../api";
import { formatBytes, formatNumber } from "../format";

interface Props {
  connectionId: string;
  onClose: () => void;
}

export function LeafNodesPanel({ connectionId, onClose }: Props) {
  const [searchTerm, setSearchTerm] = useState("");

  const leafQuery = useQuery({
    queryKey: ["leafNodes", connectionId],
    queryFn: () => api.leafNodes(connectionId),
    refetchInterval: 5000,
  });

  const leafs = leafQuery.data?.leafs ?? [];
  const count = leafQuery.data?.count ?? leafs.length;

  const filtered = leafs.filter((l: LeafNodeInfo) => {
    if (!searchTerm) return true;
    const lower = searchTerm.toLowerCase();
    return (
      l.name.toLowerCase().includes(lower) ||
      l.account.toLowerCase().includes(lower) ||
      l.ip.includes(lower)
    );
  });

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>🌿 Connected Leaf Nodes & Edge Clusters</h2>
          {leafQuery.isSuccess && (
            <span className={`pill ${count > 0 ? "ok" : "muted"}`}>
              {count} Connected
            </span>
          )}
        </div>
        <button className="link" onClick={onClose}>
          Close
        </button>
      </div>

      <div className="tail-controls" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <span className="small muted">
          Edge NATS servers bridged to this central cluster via Leaf Node protocol (port 7422)
        </span>
        <div className="tail-actions">
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search leaf name, account, IP…"
            className="small"
            style={{ width: 220 }}
          />
        </div>
      </div>

      {leafQuery.isPending && <p className="muted pad">Discovering connected leaf nodes…</p>}
      {leafQuery.isError && <div className="banner bad pad">{(leafQuery.error as Error).message}</div>}

      {leafQuery.isSuccess && count === 0 && (
        <div className="pad muted" style={{ textAlign: "center", padding: "36px 16px" }}>
          <div style={{ fontSize: 32, marginBottom: 8 }}>🌿</div>
          <h3 style={{ margin: "0 0 8px 0" }}>No Connected Leaf Nodes</h3>
          <p style={{ maxWidth: 540, margin: "0 auto", fontSize: 13, lineHeight: 1.5 }}>
            Leaf nodes extend your NATS cluster to edge locations, remote servers, or branch offices.
            When a remote NATS server connects as a leaf node (default port <code>7422</code>),
            its round-trip latency, throughput, and bridged accounts will appear here automatically.
          </p>
        </div>
      )}

      {leafQuery.isSuccess && count > 0 && filtered.length === 0 && (
        <p className="muted pad">No leaf nodes match search "{searchTerm}".</p>
      )}

      {leafQuery.isSuccess && filtered.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Leaf Node Server</th>
                <th>Role</th>
                <th>Bridged Account</th>
                <th>Address</th>
                <th>RTT Latency</th>
                <th>Subscriptions</th>
                <th className="num">Messages (In / Out)</th>
                <th className="num">Network Bytes</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((l, idx) => (
                <tr key={`${l.ip}-${l.port}-${idx}`}>
                  <td>
                    <strong>{l.name}</strong>
                  </td>
                  <td>
                    <span className={`pill ${l.isSpoke ? "warn" : "ok"}`} style={{ fontSize: 11 }}>
                      {l.isSpoke ? "Spoke (Outbound)" : "Hub (Inbound Leaf)"}
                    </span>
                  </td>
                  <td>
                    <span className="pill muted" style={{ fontFamily: "monospace" }}>
                      {l.account || "$G"}
                    </span>
                  </td>
                  <td>
                    <code>{l.ip}:{l.port}</code>
                  </td>
                  <td>
                    <span style={{ color: "var(--ok, #10b981)", fontWeight: 600 }}>
                      {l.rtt ? `🟢 ${l.rtt}` : "—"}
                    </span>
                  </td>
                  <td>
                    <span className="pill muted">{formatNumber(l.subscriptions)} active</span>
                  </td>
                  <td className="num">
                    {formatNumber(l.inMsgs)} / {formatNumber(l.outMsgs)}
                  </td>
                  <td className="num">
                    {formatBytes(l.inBytes + l.outBytes)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
