import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type ConnectionInfo } from "../api";
import { formatBytes, formatNumber, formatTime, lagLevel } from "../format";

interface Props {
  connection: ConnectionInfo;
  selectedStream: string | null;
  onSelectStream: (name: string) => void;
  onBrowseStream?: (stream: { name: string; firstSeq: number; lastSeq: number }) => void;
  onTailSubject?: (subject: string) => void;
  onViewClients?: (subject: string) => void;
  onCreateStream?: () => void;
}

export function StreamsTable({
  connection,
  selectedStream,
  onSelectStream,
  onBrowseStream,
  onTailSubject,
  onViewClients,
  onCreateStream,
}: Props) {
  const queryClient = useQueryClient();
  const [showBackingStreams, setShowBackingStreams] = useState(false);

  const streams = useQuery({
    queryKey: ["streams", connection.id],
    queryFn: () => api.streams(connection.id),
    refetchInterval: 3000,
  });

  const purge = useMutation({
    mutationFn: (stream: string) => api.purgeStream(connection.id, stream),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["streams", connection.id] }),
  });

  const del = useMutation({
    mutationFn: (stream: string) => api.deleteStream(connection.id, stream),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["streams", connection.id] }),
  });

  const handlePurge = (stream: string) => {
    if (window.confirm(`Purge all messages in stream "${stream}"? Stream config will be preserved.`)) {
      purge.mutate(stream);
    }
  };

  const handleDelete = (stream: string) => {
    if (window.confirm(`Permanently delete stream "${stream}"? This action cannot be undone.`)) {
      del.mutate(stream);
    }
  };

  const isBackingStream = (name: string) => name.startsWith("KV_") || name.startsWith("OBJ_");
  const allStreams = streams.data || [];
  const backingCount = allStreams.filter((s) => isBackingStream(s.name)).length;
  const displayedStreams = allStreams.filter(
    (s) => showBackingStreams || !isBackingStream(s.name)
  );

  const activeStreamObj = allStreams.find((s) => s.name === selectedStream);

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>Streams</h2>
          {streams.isSuccess && (
            <span className="muted small">
              {displayedStreams.length} message stream{displayedStreams.length !== 1 ? "s" : ""}
              {backingCount > 0 && !showBackingStreams && ` (${backingCount} backing hidden)`}
            </span>
          )}
        </div>
        <div style={{ display: "flex", gap: 14, alignItems: "center" }}>
          {backingCount > 0 && (
            <label
              style={{
                display: "flex",
                alignItems: "center",
                gap: 6,
                fontSize: 12,
                cursor: "pointer",
              }}
              className="muted"
            >
              <input
                type="checkbox"
                checked={showBackingStreams}
                onChange={(e) => setShowBackingStreams(e.target.checked)}
              />
              Show KV & Object backing streams
            </label>
          )}
          {onCreateStream && !connection.readOnly && (
            <button className="btn small primary" onClick={onCreateStream}>
              + Create Stream
            </button>
          )}
        </div>
      </div>

      {streams.isPending && <p className="muted pad">Loading streams…</p>}

      {streams.isError && (
        <div className="banner bad">
          {streams.error instanceof ApiError && streams.error.code === "jetstream_not_enabled"
            ? "JetStream is not enabled on this server. Start nats-server with -js."
            : (streams.error as Error).message}
        </div>
      )}

      {streams.isSuccess && displayedStreams.length === 0 && (
        <p className="muted pad">
          No message streams found. Run <code>make seed</code> or click "+ Create Stream" above.
        </p>
      )}

      {streams.isSuccess && displayedStreams.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Stream</th>
                <th>Subjects</th>
                <th className="num">Messages</th>
                <th className="num">Size</th>
                <th className="num">Consumers</th>
                <th className="num" title="Messages not yet delivered to consumers (lag)">
                  Lag
                </th>
                <th className="num" title="Delivered but not yet acknowledged">
                  Ack pending
                </th>
                <th className="num">Replicas</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {displayedStreams.map((s) => (
                <tr
                  key={s.name}
                  className={s.name === selectedStream ? "selected clickable" : "clickable"}
                  onClick={() => onSelectStream(s.name)}
                >
                  <td>
                    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                      <button className="link strong" onClick={() => onSelectStream(s.name)}>
                        {s.name}
                      </button>
                      {s.name.startsWith("KV_") && (
                        <span
                          className="pill warn"
                          style={{ fontSize: 10, padding: "1px 5px", minWidth: 20 }}
                          title="Key-Value Store backing stream"
                        >
                          KV
                        </span>
                      )}
                      {s.name.startsWith("OBJ_") && (
                        <span
                          className="pill ok"
                          style={{ fontSize: 10, padding: "1px 5px", minWidth: 20 }}
                          title="Object Store backing stream"
                        >
                          OBJ
                        </span>
                      )}
                    </div>
                  </td>
                  <td>
                    {s.subjects.map((sub) => (
                      <code key={sub} className="chip">
                        {sub}
                      </code>
                    ))}
                  </td>
                  <td className="num">{formatNumber(s.messages)}</td>
                  <td className="num">{formatBytes(s.bytes)}</td>
                  <td className="num">{s.consumerCount}</td>
                  <td className="num">
                    {s.lagError ? (
                      <span className="pill bad" title={s.lagError}>
                        error
                      </span>
                    ) : (
                      <span className={`pill ${lagLevel(s.pending)}`}>{formatNumber(s.pending)}</span>
                    )}
                  </td>
                  <td className="num">{formatNumber(s.ackPending)}</td>
                  <td className="num">{s.replicas}</td>
                  <td>
                    <div style={{ display: "flex", gap: 6, justifyContent: "flex-end", alignItems: "center" }}>
                      {onBrowseStream && (
                        <button
                          className="btn small"
                          onClick={(e) => {
                            e.stopPropagation();
                            onBrowseStream({ name: s.name, firstSeq: s.firstSeq, lastSeq: s.lastSeq });
                          }}
                        >
                          Browse
                        </button>
                      )}
                      {onViewClients && (
                        <button
                          className="btn small"
                          onClick={(e) => {
                            e.stopPropagation();
                            onViewClients(s.subjects[0] || "");
                          }}
                          title={`Clients subscribed to ${s.name}`}
                        >
                          Clients
                        </button>
                      )}
                      {onTailSubject && (
                        <button
                          className="btn small"
                          onClick={(e) => {
                            e.stopPropagation();
                            onTailSubject(s.subjects[0] || ">");
                          }}
                          title={`Tail ${s.subjects.join(", ") || "stream"}`}
                        >
                          Tail
                        </button>
                      )}
                      {!connection.readOnly && (
                        <>
                          <button
                            className="btn small"
                            onClick={(e) => {
                              e.stopPropagation();
                              handlePurge(s.name);
                            }}
                            title={`Purge all messages in ${s.name}`}
                          >
                            Purge
                          </button>
                          <button
                            className="btn small danger"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDelete(s.name);
                            }}
                            title={`Delete stream ${s.name}`}
                          >
                            ✕
                          </button>
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Stream Config & Details Banner */}
      {activeStreamObj && (
        <div
          style={{
            padding: "10px 16px",
            background: "var(--code-bg)",
            borderTop: "1px solid var(--border)",
            display: "flex",
            gap: 20,
            fontSize: 12,
            flexWrap: "wrap",
            alignItems: "center",
          }}
          className="muted"
        >
          <span>Selected: <strong>{activeStreamObj.name}</strong></span>
          <span>Storage: <code>{activeStreamObj.storage}</code></span>
          <span>Retention: <code>{activeStreamObj.retention}</code></span>
          <span>Seq Range: <code>{activeStreamObj.firstSeq}–{activeStreamObj.lastSeq}</code></span>
          {activeStreamObj.description && (
            <span>Desc: <em>{activeStreamObj.description}</em></span>
          )}
          <span>Created: <strong>{formatTime(activeStreamObj.created)}</strong></span>
        </div>
      )}
    </section>
  );
}
