import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { formatBytes, formatTime } from "../format";

interface Props {
  connectionId: string;
  readOnly?: boolean;
}

export function ObjectStorePanel({ connectionId, readOnly }: Props) {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedBucket, setSelectedBucket] = useState<string | null>(null);
  const [fileFilter, setFileFilter] = useState("");
  const [showCreateBucket, setShowCreateBucket] = useState(false);
  const [newBucketName, setNewBucketName] = useState("");
  const [newBucketDesc, setNewBucketDesc] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const buckets = useQuery({
    queryKey: ["object_buckets", connectionId],
    queryFn: () => api.listObjects(connectionId),
    refetchInterval: 5000,
  });

  const files = useQuery({
    queryKey: ["object_files", connectionId, selectedBucket],
    queryFn: () =>
      selectedBucket ? api.listObjectFiles(connectionId, selectedBucket) : Promise.resolve([]),
    enabled: !!selectedBucket,
    refetchInterval: 4000,
  });

  const createBucket = useMutation({
    mutationFn: () =>
      api.createObjectBucket(connectionId, {
        bucket: newBucketName.trim(),
        description: newBucketDesc.trim() || undefined,
      }),
    onSuccess: () => {
      setShowCreateBucket(false);
      setNewBucketName("");
      setNewBucketDesc("");
      queryClient.invalidateQueries({ queryKey: ["object_buckets", connectionId] });
    },
  });

  const deleteBucket = useMutation({
    mutationFn: (bucket: string) => api.deleteObjectBucket(connectionId, bucket),
    onSuccess: () => {
      setSelectedBucket(null);
      queryClient.invalidateQueries({ queryKey: ["object_buckets", connectionId] });
    },
  });

  const deleteFile = useMutation({
    mutationFn: (name: string) => {
      if (!selectedBucket) throw new Error("No bucket selected");
      return api.deleteObjectFile(connectionId, selectedBucket, name);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["object_files", connectionId, selectedBucket] });
      queryClient.invalidateQueries({ queryKey: ["object_buckets", connectionId] });
    },
  });

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !selectedBucket) return;
    setUploadError(null);
    setUploading(true);
    try {
      await api.uploadObject(connectionId, selectedBucket, file.name, file);
      queryClient.invalidateQueries({ queryKey: ["object_files", connectionId, selectedBucket] });
      queryClient.invalidateQueries({ queryKey: ["object_buckets", connectionId] });
    } catch (err) {
      setUploadError((err as Error).message || "Failed to upload file");
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleDeleteBucket = (b: string) => {
    if (window.confirm(`Permanently delete Object Store bucket "${b}" and all stored files?`)) {
      deleteBucket.mutate(b);
    }
  };

  const handleDeleteFile = (name: string) => {
    if (window.confirm(`Permanently delete "${name}" from bucket "${selectedBucket}"?`)) {
      deleteFile.mutate(name);
    }
  };

  const filteredFiles = (files.data || []).filter((f) =>
    f.name.toLowerCase().includes(fileFilter.toLowerCase()),
  );

  return (
    <div style={{ display: "grid", gap: 16 }}>
      {/* Buckets Card */}
      <section className="card">
        <div className="card-head">
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <h2>Object Store Buckets</h2>
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

        {buckets.isPending && <p className="muted pad">Loading Object Stores…</p>}
        {buckets.isError && <div className="banner bad">{(buckets.error as Error).message}</div>}

        {buckets.isSuccess && buckets.data.length === 0 && (
          <p className="muted pad">No Object Store buckets exist. Click "+ Create Bucket" to start.</p>
        )}

        {buckets.isSuccess && buckets.data.length > 0 && (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Bucket</th>
                  <th>Description</th>
                  <th className="num">Size</th>
                  <th>Storage</th>
                  <th className="num">Replicas</th>
                  {!readOnly && <th></th>}
                </tr>
              </thead>
              <tbody>
                {buckets.data.map((b) => (
                  <tr
                    key={b.bucket}
                    className={selectedBucket === b.bucket ? "selected clickable" : "clickable"}
                    onClick={() => setSelectedBucket(b.bucket)}
                  >
                    <td>
                      <button className="link strong">{b.bucket}</button>
                    </td>
                    <td>{b.description || <span className="muted">—</span>}</td>
                    <td className="num">{formatBytes(b.size)}</td>
                    <td><code>{b.storage}</code></td>
                    <td className="num">{b.replicas}</td>
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

      {/* Files List in Selected Bucket */}
      {selectedBucket && (
        <section className="card">
          <div className="card-head">
            <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <h2>Files in <code>{selectedBucket}</code></h2>
              {files.isSuccess && (
                <span className="muted small">{files.data.length} objects</span>
              )}
            </div>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              {!readOnly && (
                <>
                  <input
                    type="file"
                    ref={fileInputRef}
                    onChange={handleFileUpload}
                    style={{ display: "none" }}
                  />
                  <button
                    className="btn small primary"
                    disabled={uploading}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    {uploading ? "Uploading…" : "📤 Upload File"}
                  </button>
                </>
              )}
            </div>
          </div>

          {uploadError && <div className="banner bad">{uploadError}</div>}

          <div style={{ padding: "8px 16px", borderBottom: "1px solid var(--border)" }}>
            <input
              type="text"
              placeholder="Search files by name…"
              value={fileFilter}
              onChange={(e) => setFileFilter(e.target.value)}
              style={{ width: 240, padding: "4px 8px", fontSize: 12 }}
            />
          </div>

          {files.isPending && <p className="muted pad">Loading files…</p>}
          {files.isError && <div className="banner bad">{(files.error as Error).message}</div>}

          {files.isSuccess && filteredFiles.length === 0 && (
            <p className="muted pad">
              No files in this bucket. Click "Upload File" above to store an object.
            </p>
          )}

          {files.isSuccess && filteredFiles.length > 0 && (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Object Name</th>
                    <th className="num">Size</th>
                    <th>Digest</th>
                    <th>Modified Time</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {filteredFiles.map((f) => (
                    <tr key={f.name}>
                      <td>
                        <strong>{f.name}</strong>
                      </td>
                      <td className="num">{formatBytes(f.size)}</td>
                      <td>
                        {f.digest ? (
                          <code className="chip" style={{ fontSize: 11 }}>{f.digest.slice(0, 16)}…</code>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td>{formatTime(f.modTime)}</td>
                      <td>
                        <div style={{ display: "flex", gap: 6, justifyContent: "flex-end" }}>
                          <a
                            className="btn small"
                            href={api.objectDownloadUrl(connectionId, selectedBucket, f.name)}
                            download={f.name}
                            title="Download file"
                          >
                            ⬇ Download
                          </a>
                          {!readOnly && (
                            <button
                              className="btn small danger"
                              onClick={() => handleDeleteFile(f.name)}
                              title={`Delete file ${f.name}`}
                            >
                              ✕
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {/* Modal: Create Object Bucket */}
      {showCreateBucket && (
        <div className="modal-backdrop">
          <div className="modal" style={{ maxWidth: 440 }}>
            <div className="card-head">
              <h2>Create Object Store Bucket</h2>
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
                  placeholder="e.g. model-weights"
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
                  placeholder="e.g. Model binaries and file storage"
                  value={newBucketDesc}
                  onChange={(e) => setNewBucketDesc(e.target.value)}
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
    </div>
  );
}
