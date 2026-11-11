import { Fragment, useEffect, useRef, useState } from "react";
import { api, type MessageSummary } from "../api";
import { formatBytes, formatTime } from "../format";

interface Props {
  connectionId: string;
  initialSubject?: string;
  onClose: () => void;
}

const MAX_BUFFER = 200;

export function LiveTailPanel({ connectionId, initialSubject = ">", onClose }: Props) {
  const [subject, setSubject] = useState(initialSubject);
  const [activeSubject, setActiveSubject] = useState(initialSubject);
  const [isTailing, setIsTailing] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const [filter, setFilter] = useState("");
  const [messages, setMessages] = useState<MessageSummary[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [status, setStatus] = useState<"connecting" | "live" | "paused" | "stopped" | "error">("connecting");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  const scrollRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const isPausedRef = useRef(isPaused);
  isPausedRef.current = isPaused;

  useEffect(() => {
    if (!isTailing) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      setStatus("stopped");
      return;
    }

    setStatus("connecting");
    setErrorMsg(null);

    const url = api.tailUrl(connectionId, activeSubject);
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setStatus(isPausedRef.current ? "paused" : "live");
    };

    es.onmessage = (event) => {
      if (isPausedRef.current) return;
      try {
        const msg = JSON.parse(event.data) as MessageSummary;
        setMessages((prev) => {
          const next = [...prev, msg];
          return next.length > MAX_BUFFER ? next.slice(next.length - MAX_BUFFER) : next;
        });
      } catch (err) {
        console.error("Failed to parse tail event:", err);
      }
    };

    es.onerror = () => {
      setStatus("error");
      setErrorMsg("Connection to live stream dropped. Retrying…");
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [connectionId, activeSubject, isTailing]);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, autoScroll]);

  const handleApplySubject = (e: React.FormEvent) => {
    e.preventDefault();
    if (subject.trim()) {
      setActiveSubject(subject.trim());
      setIsTailing(true);
      setMessages([]);
    }
  };

  const togglePause = () => {
    const next = !isPaused;
    setIsPaused(next);
    setStatus(next ? "paused" : "live");
  };

  const filtered = messages.filter((m) => {
    if (!filter) return true;
    const lower = filter.toLowerCase();
    return (
      m.subject.toLowerCase().includes(lower) ||
      m.data.toLowerCase().includes(lower) ||
      (m.headers && Object.entries(m.headers).some(([k, v]) =>
        k.toLowerCase().includes(lower) || v.toLowerCase().includes(lower)
      ))
    );
  });

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <h2>Live Subject Tailing</h2>
          <span className={`pill ${status === "live" ? "ok" : status === "paused" ? "warn" : status === "error" ? "bad" : "muted"}`}>
            {status.toUpperCase()}
          </span>
          <span className="muted small">({messages.length} received)</span>
        </div>
        <button className="link" onClick={onClose}>
          Close
        </button>
      </div>

      <div className="tail-controls">
        <form onSubmit={handleApplySubject} className="tail-form">
          <label className="small muted" htmlFor="tail-subj">Subject pattern:</label>
          <input
            id="tail-subj"
            type="text"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            placeholder="e.g. > or dm.>"
            style={{ width: 180 }}
          />
          <button type="submit" className="btn primary small">
            Tail
          </button>
        </form>

        <div className="tail-actions">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter messages…"
            className="small"
            style={{ width: 160 }}
          />
          <button className="btn small" onClick={togglePause}>
            {isPaused ? "▶ Resume" : "⏸ Pause"}
          </button>
          <button className="btn small" onClick={() => setMessages([])}>
            Clear
          </button>
          <button
            className={`btn small ${isTailing ? "danger" : "primary"}`}
            onClick={() => setIsTailing(!isTailing)}
          >
            {isTailing ? "Stop" : "Connect"}
          </button>
          <label className="small muted" style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer" }}>
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
            />
            Auto-scroll
          </label>
        </div>
      </div>

      {errorMsg && <div className="banner bad" style={{ margin: "8px 16px" }}>{errorMsg}</div>}

      <div className="table-wrap tail-stream-wrap" ref={scrollRef}>
        {messages.length === 0 ? (
          <p className="muted pad">
            {isTailing
              ? `Waiting for messages on subject "${activeSubject}"… Try publishing messages or running make seed.`
              : "Tailing stopped. Click Connect to resume."}
          </p>
        ) : filtered.length === 0 ? (
          <p className="muted pad">No messages match filter "{filter}".</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Subject</th>
                <th className="num">Size</th>
                <th>Headers</th>
                <th>Payload Preview</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((m, idx) => {
                const rowKey = `${m.timestamp}-${idx}`;
                const isExp = expanded === rowKey;
                return (
                  <Fragment key={rowKey}>
                    <tr
                      className={isExp ? "selected clickable" : "clickable"}
                      onClick={() => setExpanded(isExp ? null : rowKey)}
                    >
                      <td className="small muted">{formatTime(m.timestamp)}</td>
                      <td>
                        <code className="chip">{m.subject}</code>
                      </td>
                      <td className="num">{formatBytes(m.size)}</td>
                      <td>
                        {m.headers ? (
                          <span className="pill warn">{Object.keys(m.headers).length}</span>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td style={{ maxWidth: 300, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        <code>{m.data.slice(0, 80)}{m.data.length > 80 ? "…" : ""}</code>
                      </td>
                      <td>
                        <button className="link small">
                          {isExp ? "▲" : "▼"}
                        </button>
                      </td>
                    </tr>
                    {isExp && (
                      <tr key={`${rowKey}-exp`}>
                        <td colSpan={6} style={{ padding: 0 }}>
                          <div className="msg-detail">
                            {m.headers && (
                              <div className="msg-headers">
                                <strong>Headers</strong>
                                <table className="kv-table">
                                  <tbody>
                                    {Object.entries(m.headers).map(([k, v]) => (
                                      <tr key={k}>
                                        <td><code>{k}</code></td>
                                        <td>{v}</td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            )}
                            <div className="msg-body">
                              <strong>Payload{m.isBinary && " (base64)"}</strong>
                              <pre className="msg-payload">{tryPrettyJson(m.data)}</pre>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

function tryPrettyJson(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

