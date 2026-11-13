import { useState } from "react";
import { api } from "../api";

interface Props {
  onSuccess: (username: string) => void;
}

export function LoginModal({ onSuccess }: Props) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin@123");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await api.login(username, password);
      onSuccess(res.username);
    } catch (err) {
      setError((err as Error).message || "Invalid credentials");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-screen">
      <div className="login-card">
        <div style={{ textAlign: "center", padding: "32px 24px 20px 24px", borderBottom: "1px solid var(--border)" }}>
          <div style={{ fontSize: 36, color: "var(--accent)", lineHeight: 1, marginBottom: 10 }}>◈</div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700 }}>NATSRadar</h1>
          <p className="muted small" style={{ margin: "6px 0 0 0" }}>Sign in to manage your NATS cluster</p>
        </div>

        <form onSubmit={handleSubmit} style={{ padding: 24, display: "grid", gap: 16 }}>
          {error && <div className="banner bad" style={{ margin: 0 }}>{error}</div>}

          <div>
            <label style={{ display: "block", fontSize: 12, marginBottom: 6, fontWeight: 600 }}>
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
              style={{ width: "100%", padding: "8px 12px" }}
            />
          </div>

          <div>
            <label style={{ display: "block", fontSize: 12, marginBottom: 6, fontWeight: 600 }}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              style={{ width: "100%", padding: "8px 12px" }}
            />
          </div>

          {/* <div
            className="muted small"
            style={{
              background: "var(--code-bg)",
              padding: "10px 12px",
              borderRadius: 6,
              lineHeight: 1.5,
            }}
          >
            Default credentials: <strong>admin</strong> / <strong>admin@123</strong>
            <br />
            Configurable via <code>NATSRADAR_AUTH_USER</code> & <code>NATSRADAR_AUTH_PASS</code>.
          </div> */}

          <button
            className="btn primary"
            type="submit"
            disabled={loading}
            style={{ width: "100%", padding: "10px 16px", fontWeight: 600, fontSize: 14 }}
          >
            {loading ? "Signing in…" : "Sign In"}
          </button>
        </form>
      </div>
    </div>
  );
}
