import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type ClientInfo } from "../api";
import { formatBytes, formatNumber } from "../format";

interface Props {
  connectionId: string;
  initialSubject?: string;
  onClose: () => void;
}

export function ClientsPanel({ connectionId, initialSubject = "", onClose }: Props) {
  const [subjectFilter, setSubjectFilter] = useState(initialSubject);
  const [activeSubject, setActiveSubject] = useState(initialSubject);
  const [searchTerm, setSearchTerm] = useState("");
  const [showInternal, setShowInternal] = useState(false);

  const clients = useQuery({
    queryKey: ["clients", connectionId, activeSubject],
    queryFn: () => api.clients(connectionId, activeSubject),
    refetchInterval: 5000,
  });

  const handleApplyFilter = (e: React.FormEvent) => {
    e.preventDefault();
    setActiveSubject(subjectFilter.trim());
  };

  const isClientInternal = (c: ClientInfo) => {
    const lower = c.name.toLowerCase();
    return (
      c.isInternal ||
      c.kind === "internal" ||
      lower === "natsradar" ||
      lower === "natscope" ||
      lower.startsWith("natsradar-") ||
      lower.startsWith("natscope-")
    );
  };

  const totalClients = clients.data?.length ?? 0;
  const internalCount = (clients.data ?? []).filter(isClientInternal).length;
  const appClientCount = totalClients - internalCount;

  const filtered = (clients.data ?? []).filter((c: ClientInfo) => {
    if (!showInternal && isClientInternal(c)) return false;
    if (!searchTerm) return true;
    const lower = searchTerm.toLowerCase();
    return (
      c.name.toLowerCase().includes(lower) ||
      c.lang.toLowerCase().includes(lower) ||
      c.ip.includes(lower) ||
      c.subscriptions.some((s) => s.toLowerCase().includes(lower))
    );
  });

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>Connected Clients & Microservices</h2>
          {clients.isSuccess && (
            <span className="muted small">
              {showInternal
                ? `${totalClients} total active connection${totalClients === 1 ? "" : "s"}`
                : `${appClientCount} microservice${appClientCount === 1 ? "" : "s"}`}
            </span>
          )}
        </div>
        <button className="link" onClick={onClose}>
          Close
        </button>
      </div>

      <div className="tail-controls">
        <form onSubmit={handleApplyFilter} className="tail-form">
          <label className="small muted" htmlFor="client-subj">Filter by Topic / Subject:</label>
          <input
            id="client-subj"
            type="text"
            value={subjectFilter}
            onChange={(e) => setSubjectFilter(e.target.value)}
            placeholder="e.g. orders.> (leave empty for all)"
            style={{ width: 220 }}
          />
          <button type="submit" className="btn small primary">
            Filter
          </button>
          {activeSubject && (
            <button
              type="button"
              className="btn small"
              onClick={() => {
                setSubjectFilter("");
                setActiveSubject("");
              }}
            >
              Clear Subject
            </button>
          )}
        </form>

        <div className="tail-actions" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {internalCount > 0 && (
            <button
              type="button"
              className={`btn small ${showInternal ? "active" : ""}`}
              onClick={() => setShowInternal(!showInternal)}
              title={showInternal ? "Hide internal system connections" : "Show internal system connections"}
              style={{ display: "flex", alignItems: "center", gap: 6 }}
            >
              <span>{showInternal ? "👁️ Hide Internal (NATSRadar)" : "👁️ Show Internal (NATSRadar)"}</span>
              <span className="pill muted" style={{ fontSize: 10, padding: "1px 5px" }}>{internalCount}</span>
            </button>
          )}
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search service name, lang…"
            className="small"
            style={{ width: 180 }}
          />
        </div>
      </div>

      {clients.isPending && <p className="muted pad">Discovering connected microservices…</p>}
      {clients.isError && <div className="banner bad pad">{(clients.error as Error).message}</div>}

      {clients.isSuccess && clients.data.length === 0 && (
        <p className="muted pad">
          {activeSubject
            ? `No microservices are currently subscribed to "${activeSubject}".`
            : "No client connections detected."}
        </p>
      )}

      {clients.isSuccess && clients.data.length > 0 && filtered.length === 0 && (
        <p className="muted pad">No clients match search "{searchTerm}".</p>
      )}

      {clients.isSuccess && filtered.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Service / Client</th>
                <th>SDK / Language</th>
                <th>Address</th>
                <th>Uptime</th>
                <th className="num">Messages (In / Out)</th>
                <th className="num">Bytes</th>
                <th>Subscribed Topics</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((c) => (
                <tr key={c.cid}>
                  <td>
                    <strong>{c.name}</strong>
                    {isClientInternal(c) && (
                      <span className="pill muted" style={{ marginLeft: 6, fontSize: 10, padding: "2px 6px" }}>
                        ⚙️ Internal
                      </span>
                    )}
                    <span className="muted small"> (#{c.cid})</span>
                  </td>
                  <td>
                    <span className="pill ok" style={{ textTransform: "capitalize" }}>
                      {c.lang || "client"} {c.version ? `v${c.version}` : ""}
                    </span>
                  </td>
                  <td>
                    <code>{c.ip}:{c.port}</code>
                  </td>
                  <td className="small muted">{c.uptime}</td>
                  <td className="num">
                    {formatNumber(c.inMsgs)} / {formatNumber(c.outMsgs)}
                  </td>
                  <td className="num">
                    {formatBytes(c.inBytes + c.outBytes)}
                  </td>
                  <td>
                    {c.subscriptions.length === 0 ? (
                      <span className="muted small">None (Publisher only)</span>
                    ) : (
                      <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                        {c.subscriptions.map((sub) => {
                          const isMatch = c.matchingSubscriptions?.includes(sub);
                          return (
                            <code
                              key={sub}
                              className="chip"
                              style={isMatch ? { background: "var(--ok-bg)", color: "var(--ok-fg)", fontWeight: 600 } : undefined}
                              title={isMatch ? "Matches target subject filter" : undefined}
                            >
                              {sub}
                            </code>
                          );
                        })}
                      </div>
                    )}
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

