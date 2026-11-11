import { useState } from "react";
import { api, type ConnectionInfo, ApiError } from "../api";

interface Props {
  connection: ConnectionInfo;
  onSuccess: (streamName: string) => void;
  onClose: () => void;
}

export function CreateStreamModal({ connection, onSuccess, onClose }: Props) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [subjectsText, setSubjectsText] = useState("");
  const [storage, setStorage] = useState<"file" | "memory">("file");
  const [retention, setRetention] = useState<"limits" | "interest" | "workqueue">("limits");
  const [replicas, setReplicas] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError("Stream name is required");
      return;
    }

    const subjects = subjectsText
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

    setLoading(true);
    setError(null);

    try {
      const created = await api.createStream(connection.id, {
        name: name.trim().toUpperCase(),
        description: description.trim() || undefined,
        subjects: subjects.length > 0 ? subjects : undefined,
        storage,
        retention,
        replicas,
      });
      onSuccess(created.name);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`[${err.code}] ${err.message}`);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("Failed to create stream");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="card" style={{ maxWidth: 540, margin: "0 auto" }}>
      <div className="card-head">
        <h2>Create JetStream Stream</h2>
        <button className="link" onClick={onClose}>
          Cancel
        </button>
      </div>

      <form onSubmit={handleSubmit} style={{ padding: 16, display: "grid", gap: 14 }}>
        <div>
          <label className="small muted" htmlFor="stream-name" style={{ display: "block", marginBottom: 4 }}>
            Stream Name *
          </label>
          <input
            id="stream-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. ORDERS, EVENTS, TELEMETRY"
            style={{ width: "100%" }}
            required
            autoFocus
          />
        </div>

        <div>
          <label className="small muted" htmlFor="stream-subj" style={{ display: "block", marginBottom: 4 }}>
            Subjects (comma separated)
          </label>
          <input
            id="stream-subj"
            type="text"
            value={subjectsText}
            onChange={(e) => setSubjectsText(e.target.value)}
            placeholder="e.g. orders.>, orders.created (defaults to NAME.>)"
            style={{ width: "100%" }}
          />
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div>
            <label className="small muted" htmlFor="stream-storage" style={{ display: "block", marginBottom: 4 }}>
              Storage Type
            </label>
            <select
              id="stream-storage"
              value={storage}
              onChange={(e) => setStorage(e.target.value as "file" | "memory")}
              style={{ width: "100%" }}
            >
              <option value="file">File (Persistent)</option>
              <option value="memory">Memory (RAM only)</option>
            </select>
          </div>

          <div>
            <label className="small muted" htmlFor="stream-retention" style={{ display: "block", marginBottom: 4 }}>
              Retention Policy
            </label>
            <select
              id="stream-retention"
              value={retention}
              onChange={(e) => setRetention(e.target.value as "limits" | "interest" | "workqueue")}
              style={{ width: "100%" }}
            >
              <option value="limits">Limits (Age / Max Msgs)</option>
              <option value="interest">Interest (Consumer driven)</option>
              <option value="workqueue">WorkQueue (Single worker)</option>
            </select>
          </div>
        </div>

        <div>
          <label className="small muted" htmlFor="stream-replicas" style={{ display: "block", marginBottom: 4 }}>
            Replicas
          </label>
          <input
            id="stream-replicas"
            type="number"
            min={1}
            max={5}
            value={replicas}
            onChange={(e) => setReplicas(Number(e.target.value))}
            style={{ width: 100 }}
          />
        </div>

        <div>
          <label className="small muted" htmlFor="stream-desc" style={{ display: "block", marginBottom: 4 }}>
            Description (optional)
          </label>
          <input
            id="stream-desc"
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="e.g. Central stream for customer orders"
            style={{ width: "100%" }}
          />
        </div>

        {error && <div className="banner bad" style={{ margin: "4px 0" }}>{error}</div>}

        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 8 }}>
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn primary" disabled={loading}>
            {loading ? "Creating…" : "Create Stream"}
          </button>
        </div>
      </form>
    </section>
  );
}

