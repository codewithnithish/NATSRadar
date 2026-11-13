import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type MessageSummary } from "./api";
import { ConnectionBar } from "./components/ConnectionBar";
import { StreamsTable } from "./components/StreamsTable";
import { ConsumersPanel } from "./components/ConsumersPanel";
import { MessagesPanel } from "./components/MessagesPanel";
import { LiveTailPanel } from "./components/LiveTailPanel";
import { PublishModal } from "./components/PublishModal";
import { ClientsPanel } from "./components/ClientsPanel";
import { LeafNodesPanel } from "./components/LeafNodesPanel";
import { CreateStreamModal } from "./components/CreateStreamModal";
import { ServerPanel } from "./components/ServerPanel";
import { KVPanel } from "./components/KVPanel";
import { ObjectStorePanel } from "./components/ObjectStorePanel";
import { LoginModal } from "./components/LoginModal";

export default function App() {
  const queryClient = useQueryClient();
  const [connectionId, setConnectionId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<
    "jetstream" | "core" | "reqrep" | "kv" | "objects" | "server"
  >("jetstream");
  const [stream, setStream] = useState<string | null>(null);
  const [browseStream, setBrowseStream] = useState<{
    name: string;
    firstSeq: number;
    lastSeq: number;
  } | null>(null);
  const [tailSubject, setTailSubject] = useState<string | null>(null);
  const [publisherSubject, setPublisherSubject] = useState<string | null>(null);
  const [publisherPayload, setPublisherPayload] = useState<string | undefined>(undefined);
  const [publisherHeaders, setPublisherHeaders] = useState<Record<string, string> | undefined>(
    undefined,
  );
  const [clientsSubject, setClientsSubject] = useState<string | null>(null);
  const [showClientsPanel, setShowClientsPanel] = useState(true);
  const [showLeafNodesPanel, setShowLeafNodesPanel] = useState(true);
  const [showLiveTailPanel, setShowLiveTailPanel] = useState(true);
  const [creatingStream, setCreatingStream] = useState(false);

  // Theme Switcher
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    const saved = localStorage.getItem("natsradar_theme") || localStorage.getItem("natscope_theme");
    if (saved === "light" || saved === "dark") return saved;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  });

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("natsradar_theme", theme);
  }, [theme]);

  const toggleTheme = () => {
    setTheme((t) => (t === "dark" ? "light" : "dark"));
  };

  // Auth Query
  const auth = useQuery({
    queryKey: ["auth_me"],
    queryFn: api.authMe,
    staleTime: 30000,
  });

  const handleLogout = async () => {
    await api.logout();
    queryClient.invalidateQueries({ queryKey: ["auth_me"] });
  };

  const connections = useQuery({
    queryKey: ["connections"],
    queryFn: api.connections,
    refetchInterval: 5000,
    enabled: !auth.data?.authEnabled || !!auth.data?.authenticated,
  });

  // Keep the selection valid
  useEffect(() => {
    const list = connections.data;
    if (!list) return;
    if (!connectionId || !list.some((c) => c.id === connectionId)) {
      setConnectionId(list[0]?.id ?? null);
      setStream(null);
      setBrowseStream(null);
      setTailSubject(null);
      setPublisherSubject(null);
      setClientsSubject(null);
    }
  }, [connections.data, connectionId]);

  const current = connections.data?.find((c) => c.id === connectionId) ?? null;

  const handleRedrive = (msg: MessageSummary) => {
    setPublisherSubject(msg.subject);
    setPublisherPayload(msg.data);
    setPublisherHeaders(msg.headers);
    setActiveTab("reqrep");
  };

  if (auth.data?.authEnabled && !auth.data?.authenticated) {
    return (
      <div className="app">
        <LoginModal onSuccess={() => queryClient.invalidateQueries({ queryKey: ["auth_me"] })} />
      </div>
    );
  }

  return (
    <div className="app">
      <header className="topbar">
        <a
          href="https://github.com/codewithnithish/NATSRadar"
          target="_blank"
          rel="noopener noreferrer"
          className="brand"
          style={{ textDecoration: "none", color: "inherit", cursor: "pointer" }}
          title="NATSRadar on GitHub"
        >
          <span className="logo" aria-hidden="true">
            ◈
          </span>
          <span>NATSRadar</span>
        </a>

        <ConnectionBar
          connections={connections.data ?? []}
          selectedId={connectionId}
          onSelect={(id) => {
            setConnectionId(id);
            setStream(null);
            setBrowseStream(null);
            setTailSubject(null);
            setPublisherSubject(null);
            setClientsSubject(null);
          }}
        />

        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          {auth.data?.authEnabled && auth.data?.authenticated && (
            <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
              <span className="muted">👤 {auth.data.username}</span>
              <button className="link danger small" onClick={handleLogout}>
                Sign out
              </button>
            </div>
          )}

          <a
            href="https://github.com/codewithnithish/NATSRadar"
            target="_blank"
            rel="noopener noreferrer"
            className="btn small"
            title="Star & View NATSRadar on GitHub"
            style={{ display: "flex", alignItems: "center", gap: 6, textDecoration: "none", color: "inherit" }}
          >
            <svg height="15" width="15" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
              <path d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z" />
            </svg>
            <span>GitHub</span>
          </a>

          <button
            className="btn small"
            onClick={toggleTheme}
            title={`Switch to ${theme === "dark" ? "Light" : "Dark"} mode`}
            style={{ display: "flex", alignItems: "center", gap: 4 }}
          >
            {theme === "dark" ? "☀️ Light" : "🌙 Dark"}
          </button>
        </div>
      </header>

      <main className="content">
        {connections.isError && (
          <div className="banner bad">
            Cannot reach the NATSRadar server: {(connections.error as Error).message}
          </div>
        )}

        {connections.isSuccess && connections.data.length === 0 && (
          <div className="empty">
            <h2>No NATS connection yet</h2>
            <p>Use “Add connection” above to point NATSRadar at a NATS server.</p>
          </div>
        )}

        {current && (
          <>
            <nav className="nav-tabs">
              <button
                className={`nav-tab ${activeTab === "jetstream" ? "active" : ""}`}
                onClick={() => setActiveTab("jetstream")}
              >
                ⚡ JetStream
              </button>
              <button
                className={`nav-tab ${activeTab === "core" ? "active" : ""}`}
                onClick={() => {
                  setShowClientsPanel(true);
                  setShowLiveTailPanel(true);
                  setActiveTab("core");
                }}
              >
                📡 Core NATS & Clients
              </button>
              <button
                className={`nav-tab ${activeTab === "reqrep" ? "active" : ""}`}
                onClick={() => {
                  setPublisherSubject(null);
                  setPublisherPayload(undefined);
                  setPublisherHeaders(undefined);
                  setActiveTab("reqrep");
                }}
              >
                ✉️ Request / Reply
              </button>
              <button
                className={`nav-tab ${activeTab === "kv" ? "active" : ""}`}
                onClick={() => setActiveTab("kv")}
              >
                🗄️ KV Store
              </button>
              <button
                className={`nav-tab ${activeTab === "objects" ? "active" : ""}`}
                onClick={() => setActiveTab("objects")}
              >
                📦 Object Store
              </button>
              <button
                className={`nav-tab ${activeTab === "server" ? "active" : ""}`}
                onClick={() => setActiveTab("server")}
              >
                ⚙️ Server & Cluster Info
              </button>
            </nav>

            {activeTab === "jetstream" && (
              <>
                <StreamsTable
                  connection={current}
                  selectedStream={stream}
                  onSelectStream={setStream}
                  onBrowseStream={setBrowseStream}
                  onTailSubject={(sub) => {
                    setTailSubject(sub);
                    setShowLiveTailPanel(true);
                    setActiveTab("core");
                  }}
                  onViewClients={(sub) => {
                    setClientsSubject(sub);
                    setShowClientsPanel(true);
                    setActiveTab("core");
                  }}
                  onCreateStream={() => setCreatingStream(true)}
                />
                {creatingStream && (
                  <CreateStreamModal
                    connection={current}
                    onSuccess={(name) => {
                      setCreatingStream(false);
                      setStream(name);
                    }}
                    onClose={() => setCreatingStream(false)}
                  />
                )}
                {browseStream && (
                  <MessagesPanel
                    connectionId={current.id}
                    stream={browseStream.name}
                    firstSeq={browseStream.firstSeq}
                    lastSeq={browseStream.lastSeq}
                    onRedrive={handleRedrive}
                    onClose={() => setBrowseStream(null)}
                  />
                )}
                {stream && (
                  <ConsumersPanel
                    connectionId={current.id}
                    stream={stream}
                    readOnly={current.readOnly}
                    onClose={() => setStream(null)}
                  />
                )}
              </>
            )}

            {activeTab === "core" && (
              <>
                {(!showClientsPanel || !showLeafNodesPanel || !showLiveTailPanel) && (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 10, marginBottom: 16 }}>
                    {!showClientsPanel && (
                      <button
                        className="btn small"
                        onClick={() => setShowClientsPanel(true)}
                      >
                        + Show Connected Clients
                      </button>
                    )}
                    {!showLeafNodesPanel && (
                      <button
                        className="btn small"
                        onClick={() => setShowLeafNodesPanel(true)}
                      >
                        + Show Leaf Nodes
                      </button>
                    )}
                    {!showLiveTailPanel && (
                      <button
                        className="btn small"
                        onClick={() => setShowLiveTailPanel(true)}
                      >
                        + Show Live Subject Tailing
                      </button>
                    )}
                  </div>
                )}
                {showClientsPanel && (
                  <ClientsPanel
                    connectionId={current.id}
                    initialSubject={clientsSubject || ""}
                    onClose={() => {
                      setClientsSubject(null);
                      setShowClientsPanel(false);
                      if (!showLiveTailPanel && !showLeafNodesPanel) {
                        setActiveTab("jetstream");
                      }
                    }}
                  />
                )}
                {showLeafNodesPanel && (
                  <LeafNodesPanel
                    connectionId={current.id}
                    onClose={() => {
                      setShowLeafNodesPanel(false);
                      if (!showClientsPanel && !showLiveTailPanel) {
                        setActiveTab("jetstream");
                      }
                    }}
                  />
                )}
                {showLiveTailPanel && (
                  <LiveTailPanel
                    connectionId={current.id}
                    initialSubject={tailSubject || ">"}
                    onClose={() => {
                      setTailSubject(null);
                      setShowLiveTailPanel(false);
                      if (!showClientsPanel && !showLeafNodesPanel) {
                        setActiveTab("jetstream");
                      }
                    }}
                  />
                )}
              </>
            )}

            {activeTab === "reqrep" && (
              <PublishModal
                connection={current}
                initialSubject={publisherSubject || ""}
                initialPayload={publisherPayload}
                initialHeaders={publisherHeaders}
                onClose={() => {
                  setPublisherSubject(null);
                  setPublisherPayload(undefined);
                  setPublisherHeaders(undefined);
                  setActiveTab("jetstream");
                }}
              />
            )}

            {activeTab === "kv" && (
              <KVPanel connectionId={current.id} readOnly={current.readOnly} />
            )}

            {activeTab === "objects" && (
              <ObjectStorePanel connectionId={current.id} readOnly={current.readOnly} />
            )}

            {activeTab === "server" && <ServerPanel connection={current} />}
          </>
        )}
      </main>
    </div>
  );
}
