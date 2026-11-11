import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { formatDuration, formatNumber, formatTime, lagLevel } from "../format";
import { CreateConsumerModal } from "./CreateConsumerModal";

interface Props {
  connectionId: string;
  stream: string;
  readOnly?: boolean;
  onClose: () => void;
}

export function ConsumersPanel({ connectionId, stream, readOnly, onClose }: Props) {
  const [showCreate, setShowCreate] = useState(false);
  const queryClient = useQueryClient();

  const consumers = useQuery({
    queryKey: ["consumers", connectionId, stream],
    queryFn: () => api.consumers(connectionId, stream),
    refetchInterval: 3000,
  });

  const del = useMutation({
    mutationFn: (consumer: string) => api.deleteConsumer(connectionId, stream, consumer),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["consumers", connectionId, stream] });
      queryClient.invalidateQueries({ queryKey: ["streams", connectionId] });
    },
  });

  const pause = useMutation({
    mutationFn: (consumer: string) => api.pauseConsumer(connectionId, stream, consumer, 60),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["consumers", connectionId, stream] }),
  });

  const resume = useMutation({
    mutationFn: (consumer: string) => api.resumeConsumer(connectionId, stream, consumer),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["consumers", connectionId, stream] }),
  });

  const handleDelete = (name: string) => {
    if (window.confirm(`Delete consumer "${name}"?`)) {
      del.mutate(name);
    }
  };

  return (
    <section className="card">
      <div className="card-head">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <h2>Consumers of <code>{stream}</code></h2>
          {consumers.isSuccess && (
            <span className="muted small">{consumers.data.length} total</span>
          )}
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          {!readOnly && (
            <button className="btn small primary" onClick={() => setShowCreate(true)}>
              + Create Consumer
            </button>
          )}
          <button className="link" onClick={onClose}>
            Close
          </button>
        </div>
      </div>

      {showCreate && (
        <CreateConsumerModal
          connectionId={connectionId}
          stream={stream}
          onSuccess={() => {
            setShowCreate(false);
            queryClient.invalidateQueries({ queryKey: ["consumers", connectionId, stream] });
            queryClient.invalidateQueries({ queryKey: ["streams", connectionId] });
          }}
          onClose={() => setShowCreate(false)}
        />
      )}

      {consumers.isPending && <p className="muted pad">Loading consumers…</p>}
      {consumers.isError && <div className="banner bad">{(consumers.error as Error).message}</div>}

      {consumers.isSuccess && consumers.data.length === 0 && (
        <p className="muted pad">This stream has no consumers. Click "+ Create Consumer" above to add one.</p>
      )}

      {consumers.isSuccess && consumers.data.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Consumer</th>
                <th>Filter</th>
                <th>Ack policy</th>
                <th className="num">Lag</th>
                <th className="num">Ack pending</th>
                <th className="num">Redelivered</th>
                <th className="num">Delivered seq</th>
                <th className="num">Ack floor</th>
                <th>Ack wait</th>
                <th className="num">Max deliver</th>
                <th>Last delivered</th>
                {!readOnly && <th></th>}
              </tr>
            </thead>
            <tbody>
              {consumers.data.map((c) => (
                <tr key={c.name}>
                  <td>
                    <strong>{c.name}</strong>
                    {!c.durable && <span className="muted small"> (ephemeral)</span>}
                  </td>
                  <td>
                    {c.filterSubjects.length === 0 ? (
                      <span className="muted">all</span>
                    ) : (
                      c.filterSubjects.map((f) => (
                        <code key={f} className="chip">
                          {f}
                        </code>
                      ))
                    )}
                  </td>
                  <td>{c.ackPolicy}</td>
                  <td className="num">
                    <span className={`pill ${lagLevel(c.pending)}`}>{formatNumber(c.pending)}</span>
                  </td>
                  <td className="num">{formatNumber(c.ackPending)}</td>
                  <td className="num">{formatNumber(c.redelivered)}</td>
                  <td className="num">{formatNumber(c.deliveredStreamSeq)}</td>
                  <td className="num">{formatNumber(c.ackFloorStreamSeq)}</td>
                  <td>{formatDuration(c.ackWaitMs)}</td>
                  <td className="num">{c.maxDeliver < 0 ? "∞" : c.maxDeliver}</td>
                  <td>{formatTime(c.lastDelivered)}</td>
                  {!readOnly && (
                    <td>
                      <div style={{ display: "flex", gap: 6, justifyContent: "flex-end" }}>
                        <button
                          className="btn small"
                          title="Pause message delivery for 1 hour"
                          onClick={() => pause.mutate(c.name)}
                        >
                          Pause
                        </button>
                        <button
                          className="btn small"
                          title="Resume message delivery"
                          onClick={() => resume.mutate(c.name)}
                        >
                          Resume
                        </button>
                        <button
                          className="btn small danger"
                          title={`Delete consumer ${c.name}`}
                          onClick={() => handleDelete(c.name)}
                        >
                          ✕
                        </button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
