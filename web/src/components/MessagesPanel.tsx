import { Fragment, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type MessageSummary } from "../api";
import { formatBytes, formatTime } from "../format";

interface Props {
  connectionId: string;
  stream: string;
  firstSeq: number;
  lastSeq: number;
  onRedrive?: (msg: MessageSummary) => void;
  onClose: () => void;
}

const PAGE_SIZE = 25;

export function MessagesPanel({ connectionId, stream, firstSeq, lastSeq, onRedrive, onClose }: Props) {
  const [startSeq, setStartSeq] = useState(firstSeq || 1);
  const [expanded, setExpanded] = useState<number | null>(null);
  const [subjectFilter, setSubjectFilter] = useState("");
  const [searchFilter, setSearchFilter] = useState("");
  const [startTime, setStartTime] = useState("");

  const messages = useQuery({
    queryKey: ["messages", connectionId, stream, startSeq, subjectFilter, searchFilter, startTime],
    queryFn: () =>
      api.messages(connectionId, stream, {
        startSeq,
        limit: PAGE_SIZE,
        subject: subjectFilter || undefined,
        search: searchFilter || undefined,
        startTime: startTime ? new Date(startTime).toISOString() : undefined,
      }),
  });

  const canPrev = startSeq > firstSeq;
  const canNext =
    messages.isSuccess && messages.data.length === PAGE_SIZE && startSeq + PAGE_SIZE <= lastSeq;

  const goPrev = () => {
    const prev = Math.max(firstSeq, startSeq - PAGE_SIZE);
    setStartSeq(prev);
    setExpanded(null);
  };
  const goNext = () => {
    setStartSeq(startSeq + PAGE_SIZE);
    setExpanded(null);
  };
  const goFirst = () => {
    setStartSeq(firstSeq);
    setExpanded(null);
  };
  const goLast = () => {
    const last = Math.max(firstSeq, lastSeq - PAGE_SIZE + 1);
    setStartSeq(last);
    setExpanded(null);
  };

  const setRelativeTime = (minutes: number) => {
    const d = new Date(Date.now() - minutes * 60 * 1000);
    setStartTime(d.toISOString().slice(0, 16));
  };

  const handleExport = () => {
    if (!messages.data || messages.data.length === 0) return;
    const blob = new Blob([JSON.stringify(messages.data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `natscope-${stream}-messages.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>
            Messages in <code>{stream}</code>
            <span className="muted small"> (seq {firstSeq}–{lastSeq})</span>
          </h2>
          {messages.isSuccess && (
            <span className="muted small">{messages.data.length} fetched</span>
          )}
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          {messages.isSuccess && messages.data.length > 0 && (
            <button className="btn small" onClick={handleExport} title="Export fetched messages to JSON file">
              📥 Export JSON
            </button>
          )}
          <button className="link" onClick={onClose}>
            Close
          </button>
        </div>
      </div>

      {/* Advanced Filter & Time-Travel Bar */}
      <div
        style={{
          padding: "10px 16px",
          background: "var(--code-bg)",
          borderBottom: "1px solid var(--border)",
          display: "flex",
          gap: 10,
          flexWrap: "wrap",
          alignItems: "center",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span className="muted small" style={{ fontWeight: 600 }}>Filter:</span>
          <input
            type="text"
            placeholder="Subject (e.g. orders.*)"
            value={subjectFilter}
            onChange={(e) => setSubjectFilter(e.target.value)}
            style={{ padding: "4px 8px", fontSize: 12, width: 140 }}
          />
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <input
            type="text"
            placeholder="Search payload or headers…"
            value={searchFilter}
            onChange={(e) => setSearchFilter(e.target.value)}
            style={{ padding: "4px 8px", fontSize: 12, width: 180 }}
          />
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span className="muted small" style={{ fontWeight: 600 }}>Time-Travel:</span>
          <input
            type="datetime-local"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
            style={{ padding: "3px 6px", fontSize: 12 }}
          />
          <button className="btn small" onClick={() => setRelativeTime(10)} title="Jump to 10 minutes ago">
            10m
          </button>
          <button className="btn small" onClick={() => setRelativeTime(60)} title="Jump to 1 hour ago">
            1h
          </button>
          {startTime && (
            <button className="btn small" onClick={() => setStartTime("")} title="Clear timestamp filter">
              ✕
            </button>
          )}
        </div>
      </div>

      {messages.isPending && <p className="muted pad">Loading messages…</p>}
      {messages.isError && <div className="banner bad">{(messages.error as Error).message}</div>}

      {messages.isSuccess && messages.data.length === 0 && (
        <p className="muted pad">
          No messages found matching criteria. Try clearing filters or adjusting sequence / time.
        </p>
      )}

      {messages.isSuccess && messages.data.length > 0 && (
        <>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th className="num">Seq</th>
                  <th>Subject</th>
                  <th className="num">Size</th>
                  <th>Timestamp</th>
                  <th>Headers</th>
                  <th></th>
                  {onRedrive && <th></th>}
                </tr>
              </thead>
              <tbody>
                {messages.data.map((m) => (
                  <Fragment key={m.seq}>
                    <tr
                      className={expanded === m.seq ? "selected clickable" : "clickable"}
                      onClick={() => setExpanded(expanded === m.seq ? null : m.seq)}
                    >
                      <td className="num">{m.seq}</td>
                      <td>
                        <code className="chip">{m.subject}</code>
                      </td>
                      <td className="num">{formatBytes(m.size)}</td>
                      <td>{formatTime(m.timestamp)}</td>
                      <td>
                        {m.headers ? (
                          <span className="pill warn">{Object.keys(m.headers).length}</span>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td>
                        <button className="link small">
                          {expanded === m.seq ? "▲" : "▼"}
                        </button>
                      </td>
                      {onRedrive && (
                        <td>
                          <button
                            className="btn small"
                            onClick={(e) => {
                              e.stopPropagation();
                              onRedrive(m);
                            }}
                            title="Re-drive / Re-publish this message to a subject"
                          >
                            ↻ Re-drive
                          </button>
                        </td>
                      )}
                    </tr>
                    {expanded === m.seq && (
                      <tr key={`${m.seq}-data`}>
                        <td colSpan={onRedrive ? 7 : 6} style={{ padding: 0 }}>
                          <div className="msg-detail">
                            {m.headers && (
                              <div className="msg-headers">
                                <strong>Headers</strong>
                                <table className="kv-table">
                                  <tbody>
                                    {Object.entries(m.headers).map(([k, v]) => (
                                      <tr key={k}>
                                        <td>
                                          <code>{k}</code>
                                        </td>
                                        <td>{v}</td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            )}
                            <div className="msg-body">
                              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                                <strong>Payload{m.isBinary && " (base64)"}</strong>
                                {onRedrive && (
                                  <button
                                    className="btn small primary"
                                    onClick={() => onRedrive(m)}
                                  >
                                    ↻ Re-drive Message
                                  </button>
                                )}
                              </div>
                              <pre className="msg-payload">{tryPrettyJson(m.data)}</pre>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>

          <div className="msg-pagination">
            <button className="btn" onClick={goFirst} disabled={!canPrev} title="First page">
              ⏮
            </button>
            <button className="btn" onClick={goPrev} disabled={!canPrev}>
              ← Prev
            </button>
            <span className="muted small">
              Showing seq {messages.data[0].seq}–{messages.data[messages.data.length - 1].seq}
            </span>
            <button className="btn" onClick={goNext} disabled={!canNext}>
              Next →
            </button>
            <button className="btn" onClick={goLast} disabled={!canNext} title="Last page">
              ⏭
            </button>
          </div>
        </>
      )}
    </section>
  );
}

/** Try to pretty-print JSON; if it fails, return the raw string. */
function tryPrettyJson(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}
