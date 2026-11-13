export interface ConnectionInfo {
  id: string;
  name: string;
  url: string;
  readOnly: boolean;
  status: string;
  serverVersion?: string;
  connectedUrl?: string;
  insecureSkipVerify?: boolean;
  hasTls?: boolean;
}

export interface StreamSummary {
  name: string;
  description?: string;
  subjects: string[];
  storage: string;
  retention: string;
  replicas: number;
  messages: number;
  bytes: number;
  firstSeq: number;
  lastSeq: number;
  consumerCount: number;
  pending: number;
  ackPending: number;
  redelivered: number;
  lagError?: string;
  created: string;
}

export interface ConsumerSummary {
  name: string;
  durable: boolean;
  description?: string;
  filterSubjects: string[];
  deliverPolicy: string;
  ackPolicy: string;
  ackWaitMs: number;
  maxDeliver: number;
  pending: number;
  ackPending: number;
  redelivered: number;
  waiting: number;
  deliveredStreamSeq: number;
  ackFloorStreamSeq: number;
  lastDelivered?: string;
  created: string;
}

export interface NewConnection {
  name: string;
  url: string;
  readOnly?: boolean;
  token?: string;
  user?: string;
  password?: string;
  insecureSkipVerify?: boolean;
  caCert?: string;
  clientCert?: string;
  clientKey?: string;
  serverName?: string;
}

export interface ServerDetails {
  serverId: string;
  serverName: string;
  version: string;
  goVersion: string;
  host: string;
  port: number;
  uptime: string;
  memBytes: number;
  cores: number;
  cpu: number;
  connections: number;
  totalConnections: number;
  subscriptions: number;
  inMsgs: number;
  outMsgs: number;
  inBytes: number;
  outBytes: number;
  maxPayload: number;
  clusterName?: string;
  leafNodes?: number;
  jetStreamEnabled: boolean;
  jsMaxMemory?: number;
  jsMaxStorage?: number;
  jsMemoryUsed?: number;
  jsStorageUsed?: number;
  jsStoreDir?: string;
}

export interface CreateStreamRequest {
  name: string;
  description?: string;
  subjects?: string[];
  storage?: "file" | "memory";
  retention?: "limits" | "interest" | "workqueue";
  replicas?: number;
}

export interface AuthInfo {
  authenticated: boolean;
  authEnabled: boolean;
  username: string;
}

export interface CreateConsumerRequest {
  name?: string;
  durable?: string;
  description?: string;
  filterSubject?: string;
  deliverPolicy?: "all" | "last" | "new" | "last_per_subject";
  ackPolicy?: "explicit" | "none" | "all";
  maxAckPending?: number;
  maxDeliver?: number;
}

export interface KVBucketSummary {
  bucket: string;
  description?: string;
  values: number;
  history: number;
  ttlMs: number;
  bytes: number;
  replicas: number;
}

export interface KVKeyEntry {
  bucket: string;
  key: string;
  value: string;
  isBinary: boolean;
  revision: number;
  created: string;
  operation: string;
}

export interface ObjectBucketSummary {
  bucket: string;
  description?: string;
  size: number;
  storage: string;
  replicas: number;
  sealed: boolean;
}

export interface ObjectSummary {
  name: string;
  size: number;
  modTime: string;
  digest?: string;
  deleted: boolean;
}

export interface MessageQueryOptions {
  startSeq?: number;
  limit?: number;
  subject?: string;
  search?: string;
  startTime?: string;
}

export interface MessageSummary {
  seq: number;
  subject: string;
  data: string;
  isBinary: boolean;
  headers?: Record<string, string>;
  size: number;
  timestamp: string;
}

export interface PublishPayload {
  subject: string;
  data: string;
  headers?: Record<string, string>;
}

export interface PublishResult {
  status: string;
  subject: string;
  bytes: number;
}

export interface RequestPayload {
  subject: string;
  data: string;
  headers?: Record<string, string>;
  timeoutMs?: number;
}

export interface RequestResult {
  subject: string;
  data: string;
  isBinary: boolean;
  headers?: Record<string, string>;
  size: number;
  roundTripMs: number;
}

export interface ClientInfo {
  cid: number;
  name: string;
  lang: string;
  version: string;
  ip: string;
  port: number;
  uptime: string;
  inMsgs: number;
  outMsgs: number;
  inBytes: number;
  outBytes: number;
  subscriptions: string[];
  matchingSubscriptions?: string[];
  isInternal?: boolean;
  kind?: string;
}

export interface LeafNodeInfo {
  account: string;
  ip: string;
  port: number;
  name: string;
  rtt?: string;
  inMsgs: number;
  outMsgs: number;
  inBytes: number;
  outBytes: number;
  subscriptions: number;
  isSpoke: boolean;
}

export interface LeafNodesSummary {
  serverId: string;
  count: number;
  leafs: LeafNodeInfo[];
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    let code = "error";
    let message = res.statusText;
    try {
      const body = await res.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? message;
    } catch {
      // body was not JSON; keep the status text
    }
    throw new ApiError(res.status, code, message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

const enc = encodeURIComponent;

export const api = {
  // Auth
  authMe: () => request<AuthInfo>("/api/auth/me"),
  login: (username: string, password: string) =>
    request<{ status: string; username: string; token: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ status: string }>("/api/auth/logout", { method: "POST" }),

  // Connections
  connections: () => request<ConnectionInfo[]>("/api/connections"),
  addConnection: (body: NewConnection) =>
    request<ConnectionInfo>("/api/connections", { method: "POST", body: JSON.stringify(body) }),
  removeConnection: (id: string) =>
    request<void>(`/api/connections/${enc(id)}`, { method: "DELETE" }),

  // Streams & Consumers
  streams: (id: string) => request<StreamSummary[]>(`/api/connections/${enc(id)}/streams`),
  createStream: (id: string, body: CreateStreamRequest) =>
    request<StreamSummary>(`/api/connections/${enc(id)}/streams`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  purgeStream: (id: string, stream: string) =>
    request<{ status: string; stream: string }>(`/api/connections/${enc(id)}/streams/${enc(stream)}/purge`, {
      method: "POST",
    }),
  deleteStream: (id: string, stream: string) =>
    request<void>(`/api/connections/${enc(id)}/streams/${enc(stream)}`, {
      method: "DELETE",
    }),

  consumers: (id: string, stream: string) =>
    request<ConsumerSummary[]>(`/api/connections/${enc(id)}/streams/${enc(stream)}/consumers`),
  createConsumer: (id: string, stream: string, body: CreateConsumerRequest) =>
    request<ConsumerSummary>(`/api/connections/${enc(id)}/streams/${enc(stream)}/consumers`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteConsumer: (id: string, stream: string, consumer: string) =>
    request<void>(`/api/connections/${enc(id)}/streams/${enc(stream)}/consumers/${enc(consumer)}`, {
      method: "DELETE",
    }),
  pauseConsumer: (id: string, stream: string, consumer: string, pauseMinutes: number = 60) =>
    request<{ status: string; consumer: string }>(
      `/api/connections/${enc(id)}/streams/${enc(stream)}/consumers/${enc(consumer)}/pause`,
      {
        method: "POST",
        body: JSON.stringify({ pauseMinutes }),
      },
    ),
  resumeConsumer: (id: string, stream: string, consumer: string) =>
    request<{ status: string; consumer: string }>(
      `/api/connections/${enc(id)}/streams/${enc(stream)}/consumers/${enc(consumer)}/resume`,
      {
        method: "POST",
      },
    ),

  // Messages with Advanced Filtering & Time Travel
  messages: (id: string, stream: string, opts?: MessageQueryOptions | number, limit?: number) => {
    const params = new URLSearchParams();
    if (typeof opts === "number") {
      if (opts) params.set("startSeq", String(opts));
      if (limit) params.set("limit", String(limit));
    } else if (opts) {
      if (opts.startSeq) params.set("startSeq", String(opts.startSeq));
      if (opts.limit) params.set("limit", String(opts.limit));
      if (opts.subject) params.set("subject", opts.subject);
      if (opts.search) params.set("search", opts.search);
      if (opts.startTime) params.set("startTime", opts.startTime);
    }
    const qs = params.toString();
    return request<MessageSummary[]>(`/api/connections/${enc(id)}/streams/${enc(stream)}/messages${qs ? `?${qs}` : ""}`);
  },

  // Key-Value (KV) Store
  listKV: (id: string) => request<KVBucketSummary[]>(`/api/connections/${enc(id)}/kv`),
  createKV: (id: string, body: { bucket: string; description?: string; history?: number; ttlSeconds?: number }) =>
    request<{ status: string; bucket: string }>(`/api/connections/${enc(id)}/kv`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteKV: (id: string, bucket: string) =>
    request<void>(`/api/connections/${enc(id)}/kv/${enc(bucket)}`, { method: "DELETE" }),
  listKVKeys: (id: string, bucket: string) =>
    request<string[]>(`/api/connections/${enc(id)}/kv/${enc(bucket)}/keys`),
  getKVKey: (id: string, bucket: string, key: string) =>
    request<KVKeyEntry>(`/api/connections/${enc(id)}/kv/${enc(bucket)}/keys/${enc(key)}`),
  putKVKey: (id: string, bucket: string, key: string, value: string) =>
    request<{ status: string; revision: number }>(`/api/connections/${enc(id)}/kv/${enc(bucket)}/keys/${enc(key)}`, {
      method: "POST",
      body: JSON.stringify({ value }),
    }),
  deleteKVKey: (id: string, bucket: string, key: string) =>
    request<void>(`/api/connections/${enc(id)}/kv/${enc(bucket)}/keys/${enc(key)}`, {
      method: "DELETE",
    }),
  getKVHistory: (id: string, bucket: string, key: string) =>
    request<KVKeyEntry[]>(`/api/connections/${enc(id)}/kv/${enc(bucket)}/keys/${enc(key)}/history`),

  // Object Store
  listObjects: (id: string) => request<ObjectBucketSummary[]>(`/api/connections/${enc(id)}/objects`),
  createObjectBucket: (id: string, body: { bucket: string; description?: string }) =>
    request<{ status: string; bucket: string }>(`/api/connections/${enc(id)}/objects`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteObjectBucket: (id: string, bucket: string) =>
    request<void>(`/api/connections/${enc(id)}/objects/${enc(bucket)}`, { method: "DELETE" }),
  listObjectFiles: (id: string, bucket: string) =>
    request<ObjectSummary[]>(`/api/connections/${enc(id)}/objects/${enc(bucket)}/list`),
  objectDownloadUrl: (id: string, bucket: string, name: string) =>
    `/api/connections/${enc(id)}/objects/${enc(bucket)}/get?name=${enc(name)}`,
  uploadObject: async (id: string, bucket: string, name: string, fileData: Blob | ArrayBuffer | string) => {
    const res = await fetch(`/api/connections/${enc(id)}/objects/${enc(bucket)}/put?name=${enc(name)}`, {
      method: "POST",
      body: fileData,
    });
    if (!res.ok) throw new Error("Upload failed: " + res.statusText);
    return res.json();
  },
  deleteObjectFile: (id: string, bucket: string, name: string) =>
    request<void>(`/api/connections/${enc(id)}/objects/${enc(bucket)}/delete?name=${enc(name)}`, {
      method: "DELETE",
    }),

  // Core & Real-time
  tailUrl: (id: string, subject?: string) => {
    const qs = subject ? `?subject=${enc(subject)}` : "";
    return `/api/connections/${enc(id)}/tail${qs}`;
  },
  publish: (id: string, payload: PublishPayload) =>
    request<PublishResult>(`/api/connections/${enc(id)}/publish`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  request: (id: string, payload: RequestPayload) =>
    request<RequestResult>(`/api/connections/${enc(id)}/request`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  clients: (id: string, subject?: string) => {
    const qs = subject ? `?subject=${enc(subject)}` : "";
    return request<ClientInfo[]>(`/api/connections/${enc(id)}/clients${qs}`);
  },
  leafNodes: (id: string) => request<LeafNodesSummary>(`/api/connections/${enc(id)}/leafs`),
  serverDetails: (id: string) => request<ServerDetails>(`/api/connections/${enc(id)}/server`),
};
