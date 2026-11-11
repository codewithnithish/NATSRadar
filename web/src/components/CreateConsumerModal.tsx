import { useState } from "react";
import { api, type CreateConsumerRequest } from "../api";

interface Props {
  connectionId: string;
  stream: string;
  onSuccess: (name: string) => void;
  onClose: () => void;
}

export function CreateConsumerModal({ connectionId, stream, onSuccess, onClose }: Props) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [filterSubject, setFilterSubject] = useState("");
  const [deliverPolicy, setDeliverPolicy] = useState<"all" | "last" | "new" | "last_per_subject">("all");
  const [ackPolicy, setAckPolicy] = useState<"explicit" | "none" | "all">("explicit");
  const [maxDeliver, setMaxDeliver] = useState(5);
  const [maxAckPending, setMaxAckPending] = useState(1000);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError("Consumer name is required");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const payload: CreateConsumerRequest = {
        name: name.trim(),
        durable: name.trim(),
        description: description.trim() || undefined,
        filterSubject: filterSubject.trim() || undefined,
        deliverPolicy,
        ackPolicy,
        maxDeliver: Number(maxDeliver) || 5,
        maxAckPending: Number(maxAckPending) || 1000,
      };
      await api.createConsumer(connectionId, stream, payload);
      onSuccess(name.trim());
    } catch (err) {
      setError((err as Error).message || "Failed to create consumer");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-backdrop">
      <div className="modal" style={{ maxWidth: 520 }}>
        <div className="card-head">
          <h2>Create Consumer for <code>{stream}</code></h2>
          <button className="link" onClick={onClose}>✕</button>
        </div>
        <form onSubmit={handleSubmit} style={{ padding: 20, display: "grid", gap: 14 }}>
          {error && <div className="banner bad" style={{ margin: 0 }}>{error}</div>}

          <div>
            <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
              Consumer / Durable Name *
            </label>
            <input
              type="text"
              placeholder="e.g. orders-worker-group"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              autoFocus
              style={{ width: "100%" }}
            />
          </div>

          <div>
            <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
              Filter Subject (optional)
            </label>
            <input
              type="text"
              placeholder="e.g. orders.us (leave blank for all stream subjects)"
              value={filterSubject}
              onChange={(e) => setFilterSubject(e.target.value)}
              style={{ width: "100%" }}
            />
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div>
              <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                Deliver Policy
              </label>
              <select
                value={deliverPolicy}
                onChange={(e) => setDeliverPolicy(e.target.value as any)}
                style={{ width: "100%" }}
              >
                <option value="all">All (From stream start)</option>
                <option value="last">Last (Most recent msg)</option>
                <option value="new">New (Only future msgs)</option>
                <option value="last_per_subject">Last Per Subject</option>
              </select>
            </div>

            <div>
              <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                Ack Policy
              </label>
              <select
                value={ackPolicy}
                onChange={(e) => setAckPolicy(e.target.value as any)}
                style={{ width: "100%" }}
              >
                <option value="explicit">Explicit (Per-message Ack)</option>
                <option value="none">None (Auto-ack on deliver)</option>
                <option value="all">All (Cumulative Ack)</option>
              </select>
            </div>
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div>
              <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                Max Deliver (Redeliveries)
              </label>
              <input
                type="number"
                min={1}
                max={100}
                value={maxDeliver}
                onChange={(e) => setMaxDeliver(Number(e.target.value))}
                style={{ width: "100%" }}
              />
            </div>

            <div>
              <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                Max Ack Pending
              </label>
              <input
                type="number"
                min={1}
                max={100000}
                value={maxAckPending}
                onChange={(e) => setMaxAckPending(Number(e.target.value))}
                style={{ width: "100%" }}
              />
            </div>
          </div>

          <div>
            <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
              Description
            </label>
            <input
              type="text"
              placeholder="e.g. Microservice payment processor pull consumer"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              style={{ width: "100%" }}
            />
          </div>

          <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 8 }}>
            <button className="btn" type="button" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button className="btn primary" type="submit" disabled={submitting}>
              {submitting ? "Creating…" : "Create Consumer"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

