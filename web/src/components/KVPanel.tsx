import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { formatBytes, formatTime } from "../format";

interface Props {
  connectionId: string;
  readOnly?: boolean;
}

export function KVPanel({ connectionId, readOnly }: Props) {
  const queryClient = useQueryClient();
  const [selectedBucket, setSelectedBucket] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [keyFilter, setKeyFilter] = useState("");
  const [showCreateBucket, setShowCreateBucket] = useState(false);
  const [showPutKey, setShowPutKey] = useState(false);
  const [showHistory, setShowHistory] = useState(false);

  // Form states
  const [newBucketName, setNewBucketName] = useState("");
  const [newBucketDesc, setNewBucketDesc] = useState("");
  const [newBucketHistory, setNewBucketHistory] = useState(5);
  const [putKeyName, setPutKeyName] = useState("");
  const [putKeyValue, setPutKeyValue] = useState("");

  const buckets = useQuery({
    queryKey: ["kv_buckets", connectionId],
    queryFn: () => api.listKV(connectionId),
    refetchInterval: 5000,
  });

  const keys = useQuery({
    queryKey: ["kv_keys", connectionId, selectedBucket],
    queryFn: () => (selectedBucket ? api.listKVKeys(connectionId, selectedBucket) : Promise.resolve([])),
    enabled: !!selectedBucket,
    refetchInterval: 4000,
  });

  const keyDetail = useQuery({
    queryKey: ["kv_key", connectionId, selectedBucket, selectedKey],
    queryFn: () =>
      selectedBucket && selectedKey ? api.getKVKey(connectionId, selectedBucket, selectedKey) : null,
    enabled: !!selectedBucket && !!selectedKey,
  });

  const keyHistory = useQuery({
    queryKey: ["kv_history", connectionId, selectedBucket, selectedKey],
    queryFn: () =>
      selectedBucket && selectedKey ? api.getKVHistory(connectionId, selectedBucket, selectedKey) : [],
    enabled: !!selectedBucket && !!selectedKey && showHistory,
  });

  const createBucket = useMutation({
    mutationFn: () =>
      api.createKV(connectionId, {
        bucket: newBucketName.trim(),
        description: newBucketDesc.trim() || undefined,
        history: Number(newBucketHistory) || 5,
      }),
    onSuccess: () => {
      setShowCreateBucket(false);
      setNewBucketName("");
      setNewBucketDesc("");
      queryClient.invalidateQueries({ queryKey: ["kv_buckets", connectionId] });
    },
  });

  const deleteBucket = useMutation({
    mutationFn: (bucket: string) => api.deleteKV(connectionId, bucket),
    onSuccess: () => {
      setSelectedBucket(null);
      setSelectedKey(null);
      queryClient.invalidateQueries({ queryKey: ["kv_buckets", connectionId] });
    },
  });

  const putKey = useMutation({
    mutationFn: () => {
      if (!selectedBucket) throw new Error("No bucket selected");
      return api.putKVKey(connectionId, selectedBucket, putKeyName.trim(), putKeyValue);
    },
    onSuccess: () => {
      setShowPutKey(false);
      setSelectedKey(putKeyName.trim());
      setPutKeyName("");
      setPutKeyValue("");
      queryClient.invalidateQueries({ queryKey: ["kv_keys", connectionId, selectedBucket] });
      queryClient.invalidateQueries({ queryKey: ["kv_key", connectionId, selectedBucket] });
      queryClient.invalidateQueries({ queryKey: ["kv_buckets", connectionId] });
    },
  });

  const deleteKey = useMutation({
    mutationFn: (key: string) => {
      if (!selectedBucket) throw new Error("No bucket selected");
      return api.deleteKVKey(connectionId, selectedBucket, key);
    },
    onSuccess: () => {
      setSelectedKey(null);
      queryClient.invalidateQueries({ queryKey: ["kv_keys", connectionId, selectedBucket] });
      queryClient.invalidateQueries({ queryKey: ["kv_buckets", connectionId] });
    },
  });

  const handleDeleteBucket = (b: string) => {
    if (window.confirm(`Permanently delete KV bucket "${b}" and all its keys?`)) {
      deleteBucket.mutate(b);
    }
  };

  const handleDeleteKey = (k: string) => {
    if (window.confirm(`Delete key "${k}" from bucket "${selectedBucket}"?`)) {
      deleteKey.mutate(k);
    }
  };

  const filteredKeys = (keys.data || []).filter((k) =>
    k.toLowerCase().includes(keyFilter.toLowerCase()),
  );

  return (
    <div style={{ display: "grid", gap: 16 }}>
      {/* Buckets Card */}
      <section className="card">
        <div className="card-head">
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <h2>Key-Value Buckets</h2>
            {buckets.isSuccess && (
              <span className="muted small">{buckets.data.length} total</span>
            )}
          </div>
          {!readOnly && (
            <button className="btn small primary" onClick={() => setShowCreateBucket(true)}>
              + Create Bucket
            </button>
          )}
        </div>

        {buckets.isPending && <p className="muted pad">Loading KV stores…</p>}
        {buckets.isError && <div className="banner bad">{(buckets.error as Error).message}</div>}

        {buckets.isSuccess && buckets.data.length === 0 && (
          <p className="muted pad">No Key-Value buckets exist. Click "+ Create Bucket" to start.</p>
        )}

        {buckets.isSuccess && buckets.data.length > 0 && (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Bucket</th>
                  <th>Description</th>
                  <th className="num">Values</th>
                  <th className="num">History Depth</th>
                  <th className="num">Size</th>
                  {!readOnly && <th></th>}
                </tr>
              </thead>
              <tbody>
                {buckets.data.map((b) => (
                  <tr
                    key={b.bucket}
                    className={selectedBucket === b.bucket ? "selected clickable" : "clickable"}
                    onClick={() => {
                      setSelectedBucket(b.bucket);
                      setSelectedKey(null);
                    }}
                  >
                    <td>
                      <button className="link strong">{b.bucket}</button>
                    </td>
                    <td>{b.description || <span className="muted">—</span>}</td>
                    <td className="num">{b.values}</td>
                    <td className="num">{b.history}</td>
                    <td className="num">{formatBytes(b.bytes)}</td>
                    {!readOnly && (
                      <td>
                        <div style={{ display: "flex", justifyContent: "flex-end" }}>
                          <button
                            className="btn small danger"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDeleteBucket(b.bucket);
                            }}
                            title={`Delete bucket ${b.bucket}`}
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

      {/* Keys & Value Browser for Selected Bucket */}
      {selectedBucket && (
        <div style={{ display: "grid", gridTemplateColumns: "320px 1fr", gap: 16 }}>
          {/* Key List Column */}
          <section className="card">
            <div className="card-head">
              <h2>Keys in <code>{selectedBucket}</code></h2>
              {!readOnly && (
                <button
                  className="btn small primary"
                  onClick={() => {
                    setPutKeyName("");
                    setPutKeyValue("");
                    setShowPutKey(true);
                  }}
                >
                  + Put Key
                </button>
              )}
            </div>
            <div style={{ padding: "8px 12px", borderBottom: "1px solid var(--border)" }}>
              <input
                type="text"
                placeholder="Filter keys…"
                value={keyFilter}
                onChange={(e) => setKeyFilter(e.target.value)}
                style={{ width: "100%", padding: "4px 8px", fontSize: 12 }}
              />
            </div>
            <div style={{ maxHeight: 500, overflowY: "auto" }}>
              {keys.isPending && <p className="muted pad">Loading keys…</p>}
              {keys.isSuccess && filteredKeys.length === 0 && (
                <p className="muted pad small">No keys found.</p>
              )}
              {keys.isSuccess && (
                <ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
                  {filteredKeys.map((k) => (
                    <li
                      key={k}
                      style={{
                        padding: "8px 12px",
                        borderBottom: "1px solid var(--border)",
                        cursor: "pointer",
                        background: selectedKey === k ? "var(--row-selected)" : "transparent",
                        fontWeight: selectedKey === k ? 600 : "normal",
                      }}
                      onClick={() => {
                        setSelectedKey(k);
                        setShowHistory(false);
                      }}
                    >
                      <code>{k}</code>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </section>

          {/* Key Value & Inspector Column */}
          <section className="card">
            <div className="card-head">
              <h2>
                {selectedKey ? (
                  <>Key: <code>{selectedKey}</code></>
                ) : (
                  "Select a key to view contents"
                )}
              </h2>
              {selectedKey && (
                <div style={{ display: "flex", gap: 8 }}>
                  <button className="btn small" onClick={() => setShowHistory(!showHistory)}>
                    {showHistory ? "View Current" : "📜 Revision History"}
                  </button>
                  {!readOnly && (
                    <>
                      <button
                        className="btn small"
                        onClick={() => {
                          setPutKeyName(selectedKey);
                          setPutKeyValue(keyDetail.data?.value || "");
                          setShowPutKey(true);
                        }}
                      >
                        Edit
                      </button>
                      <button
                        className="btn small danger"
                        onClick={() => handleDeleteKey(selectedKey)}
                      >
                        Delete
                      </button>
                    </>
                  )}
                </div>
              )}
            </div>

            <div style={{ padding: 16 }}>
              {!selectedKey && (
                <p className="muted">Click on any key in the left list to inspect its value and revision metadata.</p>
              )}

              {selectedKey && keyDetail.isPending && <p className="muted">Loading value…</p>}
              {selectedKey && keyDetail.isError && (
                <div className="banner bad">{(keyDetail.error as Error).message}</div>
              )}

              {selectedKey && keyDetail.isSuccess && keyDetail.data && !showHistory && (
                <div style={{ display: "grid", gap: 12 }}>
                  <div style={{ display: "flex", gap: 16, fontSize: 12 }} className="muted">
                    <span>Revision: <strong>#{keyDetail.data.revision}</strong></span>
                    <span>Operation: <span className="pill ok">{keyDetail.data.operation}</span></span>
                    <span>Updated: <strong>{formatTime(keyDetail.data.created)}</strong></span>
                  </div>
                  <div>
                    <label style={{ display: "block", fontSize: 12, fontWeight: 600, marginBottom: 4 }}>
                      Value {keyDetail.data.isBinary && "(base64 encoded)"}
                    </label>
                    <pre className="msg-payload" style={{ maxHeight: 380, overflowY: "auto" }}>
                      {tryPrettyJson(keyDetail.data.value)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Revision History View */}
              {selectedKey && showHistory && (
                <div>
                  <h3 style={{ fontSize: 14, margin: "0 0 10px 0" }}>
                    Revision History for <code>{selectedKey}</code>
                  </h3>
                  {keyHistory.isPending && <p className="muted">Loading revisions…</p>}
                  {keyHistory.isSuccess && (
                    <div className="table-wrap">
                      <table>
                        <thead>
                          <tr>
                            <th>Revision</th>
                            <th>Operation</th>
                            <th>Timestamp</th>
                            <th>Value</th>
                          </tr>
                        </thead>
                        <tbody>
                          {keyHistory.data.map((h) => (
                            <tr key={h.revision}>
                              <td><strong>#{h.revision}</strong></td>
                              <td><span className={`pill ${h.operation === "DEL" ? "bad" : "ok"}`}>{h.operation}</span></td>
                              <td>{formatTime(h.created)}</td>
                              <td>
                                <code style={{ fontSize: 11 }}>{h.value.slice(0, 80)}{h.value.length > 80 ? "…" : ""}</code>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              )}
            </div>
          </section>
        </div>
      )}

      {/* Modal: Create Bucket */}
      {showCreateBucket && (
        <div className="modal-backdrop">
          <div className="modal" style={{ maxWidth: 440 }}>
            <div className="card-head">
              <h2>Create Key-Value Bucket</h2>
              <button className="link" onClick={() => setShowCreateBucket(false)}>✕</button>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                createBucket.mutate();
              }}
              style={{ padding: 20, display: "grid", gap: 14 }}
            >
              <div>
                <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                  Bucket Name *
                </label>
                <input
                  type="text"
                  placeholder="e.g. app-configs"
                  value={newBucketName}
                  onChange={(e) => setNewBucketName(e.target.value)}
                  required
                  autoFocus
                  style={{ width: "100%" }}
                />
              </div>
              <div>
                <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                  Description
                </label>
                <input
                  type="text"
                  placeholder="e.g. Dynamic service configs"
                  value={newBucketDesc}
                  onChange={(e) => setNewBucketDesc(e.target.value)}
                  style={{ width: "100%" }}
                />
              </div>
              <div>
                <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                  History Depth (revisions kept per key)
                </label>
                <input
                  type="number"
                  min={1}
                  max={64}
                  value={newBucketHistory}
                  onChange={(e) => setNewBucketHistory(Number(e.target.value))}
                  style={{ width: "100%" }}
                />
              </div>
              <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 6 }}>
                <button className="btn" type="button" onClick={() => setShowCreateBucket(false)}>
                  Cancel
                </button>
                <button className="btn primary" type="submit" disabled={createBucket.isPending}>
                  {createBucket.isPending ? "Creating…" : "Create Bucket"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Put / Edit Key */}
      {showPutKey && (
        <div className="modal-backdrop">
          <div className="modal" style={{ maxWidth: 540 }}>
            <div className="card-head">
              <h2>Put Key in <code>{selectedBucket}</code></h2>
              <button className="link" onClick={() => setShowPutKey(false)}>✕</button>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                putKey.mutate();
              }}
              style={{ padding: 20, display: "grid", gap: 14 }}
            >
              <div>
                <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                  Key Name *
                </label>
                <input
                  type="text"
                  placeholder="e.g. feature.enable_v2"
                  value={putKeyName}
                  onChange={(e) => setPutKeyName(e.target.value)}
                  required
                  autoFocus
                  style={{ width: "100%" }}
                />
              </div>
              <div>
                <label style={{ display: "block", fontSize: 12, marginBottom: 4, fontWeight: 600 }}>
                  Value (Text or JSON)
                </label>
                <textarea
                  rows={8}
                  placeholder='{"enabled": true, "threshold": 50}'
                  value={putKeyValue}
                  onChange={(e) => setPutKeyValue(e.target.value)}
                  style={{
                    width: "100%",
                    fontFamily: "ui-monospace, monospace",
                    fontSize: 13,
                    padding: 8,
                    borderRadius: 6,
                    border: "1px solid var(--border)",
                    background: "var(--surface)",
                    color: "var(--text)",
                  }}
                />
              </div>
              <div style={{ display: "flex", justifyContent: "flex-end", gap: 10 }}>
                <button className="btn" type="button" onClick={() => setShowPutKey(false)}>
                  Cancel
                </button>
                <button className="btn primary" type="submit" disabled={putKey.isPending}>
                  {putKey.isPending ? "Saving…" : "Save Key"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function tryPrettyJson(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}
