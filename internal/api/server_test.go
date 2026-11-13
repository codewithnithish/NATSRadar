package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/codewithnithish/natsradar/internal/api"
	"github.com/codewithnithish/natsradar/internal/conn"
	"github.com/codewithnithish/natsradar/internal/testutil"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := conn.NewManager(log)
	t.Cleanup(mgr.Close)

	ui := fstest.MapFS{"index.html": {Data: []byte("<html>natscope-ui</html>")}}
	ts := httptest.NewServer(api.New(mgr, ui, "test", log))
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestHealth(t *testing.T) {
	ts := newTestServer(t)
	res, body := do(t, "GET", ts.URL+"/api/health", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("health: %d %s", res.StatusCode, body)
	}
}

func TestSPAFallbackAndAPINotFound(t *testing.T) {
	ts := newTestServer(t)

	res, body := do(t, "GET", ts.URL+"/streams/DM", "")
	if res.StatusCode != 200 || !strings.Contains(body, "natscope-ui") {
		t.Errorf("SPA fallback: %d %q", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/nope", "")
	if res.StatusCode != 404 || !strings.Contains(body, "not_found") {
		t.Errorf("unknown API path: %d %q", res.StatusCode, body)
	}
}

func TestConnectionValidation(t *testing.T) {
	ts := newTestServer(t)

	res, _ := do(t, "POST", ts.URL+"/api/connections", `{not json`)
	if res.StatusCode != 400 {
		t.Errorf("bad JSON: got %d, want 400", res.StatusCode)
	}
	res, body := do(t, "POST", ts.URL+"/api/connections", `{"name":"x"}`)
	if res.StatusCode != 400 || !strings.Contains(body, "invalid_connection") {
		t.Errorf("missing url: %d %s", res.StatusCode, body)
	}
	res, body = do(t, "GET", ts.URL+"/api/connections/missing/streams", "")
	if res.StatusCode != 404 || !strings.Contains(body, "connection_not_found") {
		t.Errorf("unknown connection: %d %s", res.StatusCode, body)
	}

	// Invalid NKey seed should be rejected with 400
	res, body = do(t, "POST", ts.URL+"/api/connections", `{"name":"bad-nkey","url":"nats://127.0.0.1:4222","nkey":"not-a-valid-seed"}`)
	if res.StatusCode != 400 || !strings.Contains(body, "invalid_connection") {
		t.Errorf("invalid nkey: got %d %s, want 400", res.StatusCode, body)
	}

	// Invalid credentials content should be rejected with 400
	res, body = do(t, "POST", ts.URL+"/api/connections", `{"name":"bad-creds","url":"nats://127.0.0.1:4222","credentials":"not-valid-creds"}`)
	if res.StatusCode != 400 || !strings.Contains(body, "invalid_connection") {
		t.Errorf("invalid creds: got %d %s, want 400", res.StatusCode, body)
	}
}

func TestConnectionAndStreamsEndToEnd(t *testing.T) {
	natsURL := testutil.RunNATS(t)
	ts := newTestServer(t)

	// Add a connection.
	res, body := do(t, "POST", ts.URL+"/api/connections", `{"name":"Local Dev","url":"`+natsURL+`"}`)
	if res.StatusCode != 201 {
		t.Fatalf("add connection: %d %s", res.StatusCode, body)
	}
	var info conn.Info
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.ID != "local-dev" || info.Status != "CONNECTED" {
		t.Errorf("unexpected connection info: %+v", info)
	}

	// Duplicate id is rejected.
	res, _ = do(t, "POST", ts.URL+"/api/connections", `{"name":"Local Dev","url":"`+natsURL+`"}`)
	if res.StatusCode != 409 {
		t.Errorf("duplicate: got %d, want 409", res.StatusCode)
	}

	// Empty server: streams is an empty JSON array, not null.
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/streams", "")
	if res.StatusCode != 200 || strings.TrimSpace(body) != "[]" {
		t.Fatalf("empty streams: %d %q", res.StatusCode, body)
	}

	// Create a stream on the server, then see it through the API.
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name: "ORDERS", Subjects: []string{"orders.>"}, Storage: jetstream.MemoryStorage,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish(ctx, "orders.created", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/streams", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"name":"ORDERS"`) || !strings.Contains(body, `"messages":1`) {
		t.Fatalf("streams: %d %s", res.StatusCode, body)
	}

	// Fetch messages from the stream.
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/streams/ORDERS/messages?startSeq=1&limit=10", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"seq":1`) || !strings.Contains(body, `"subject":"orders.created"`) {
		t.Fatalf("messages: %d %s", res.StatusCode, body)
	}

	// Test live tailing endpoint
	tailCtx, tailCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer tailCancel()
	tailReq, _ := http.NewRequestWithContext(tailCtx, "GET", ts.URL+"/api/connections/local-dev/tail?subject=orders.>", nil)
	tailRes, err := http.DefaultClient.Do(tailReq)
	if err != nil {
		t.Fatalf("tail request: %v", err)
	}
	if tailRes.StatusCode != 200 || !strings.Contains(tailRes.Header.Get("Content-Type"), "text/event-stream") {
		tailRes.Body.Close()
		t.Fatalf("tail headers: status %d content-type %s", tailRes.StatusCode, tailRes.Header.Get("Content-Type"))
	}
	tailRes.Body.Close()

	// Test Publish
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/publish", `{"subject":"test.publish","data":"hello"}`)
	if res.StatusCode != 200 || !strings.Contains(body, `"status":"published"`) {
		t.Fatalf("publish: %d %s", res.StatusCode, body)
	}

	// Test Request/Reply with an active responder
	sub, err := nc.Subscribe("service.echo", func(m *nats.Msg) {
		_ = m.Respond([]byte("echo: " + string(m.Data)))
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/request", `{"subject":"service.echo","data":"ping"}`)
	if res.StatusCode != 200 || !strings.Contains(body, `"echo: ping"`) {
		t.Fatalf("request: %d %s", res.StatusCode, body)
	}

	// Test Request timeout (no responders listening on subject)
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/request", `{"subject":"no.one.home","data":"ping","timeoutMs":100}`)
	if res.StatusCode != 502 && res.StatusCode != 504 {
		t.Fatalf("expected timeout/no_responder status, got %d %s", res.StatusCode, body)
	}

	// Test ReadOnly connection enforcement
	_, _ = do(t, "POST", ts.URL+"/api/connections", `{"name":"RO","url":"`+natsURL+`","readOnly":true}`)
	res, body = do(t, "POST", ts.URL+"/api/connections/ro/publish", `{"subject":"test.ro","data":"hello"}`)
	if res.StatusCode != 403 || !strings.Contains(body, `"read_only"`) {
		t.Fatalf("expected 403 read_only for publish, got %d %s", res.StatusCode, body)
	}
	res, body = do(t, "POST", ts.URL+"/api/connections/ro/request", `{"subject":"test.ro","data":"hello"}`)
	if res.StatusCode != 403 || !strings.Contains(body, `"read_only"`) {
		t.Fatalf("expected 403 read_only for request, got %d %s", res.StatusCode, body)
	}

	// Test List Clients (Topic Subscribers)
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/clients?subject=service.echo", "")
	if res.StatusCode != 200 || !strings.Contains(body, "service.echo") {
		t.Fatalf("list clients: %d %s", res.StatusCode, body)
	}

	// Test List Leaf Nodes
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/leafs", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"count"`) {
		t.Fatalf("list leaf nodes: %d %s", res.StatusCode, body)
	}

	// Test Server Details
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/server", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"version"`) {
		t.Fatalf("server details: %d %s", res.StatusCode, body)
	}

	// Test Create, Purge, and Delete Stream
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/streams", `{"name":"TEMP_STREAM","subjects":["temp.>"],"storage":"memory"}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"name":"TEMP_STREAM"`) {
		t.Fatalf("create stream: %d %s", res.StatusCode, body)
	}

	// Publish to TEMP_STREAM
	_, _ = do(t, "POST", ts.URL+"/api/connections/local-dev/publish", `{"subject":"temp.1","data":"foo"}`)

	// Purge TEMP_STREAM
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/streams/TEMP_STREAM/purge", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"status":"purged"`) {
		t.Fatalf("purge stream: %d %s", res.StatusCode, body)
	}

	// Delete TEMP_STREAM
	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/streams/TEMP_STREAM", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete stream: %d", res.StatusCode)
	}

	// Consumers of an unknown stream -> 404.
	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/streams/NOPE/consumers", "")
	if res.StatusCode != 404 || !strings.Contains(body, "stream_not_found") {
		t.Errorf("unknown stream: %d %s", res.StatusCode, body)
	}

	// Test Consumer Management (Create, Pause, Resume, Delete)
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/streams/ORDERS/consumers",
		`{"name":"orders-audit","durable":"orders-audit","deliverPolicy":"all","ackPolicy":"explicit"}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"name":"orders-audit"`) {
		t.Fatalf("create consumer: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/streams/ORDERS/consumers/orders-audit/pause", `{"pauseMinutes":10}`)
	if res.StatusCode != 200 || !strings.Contains(body, `"paused"`) {
		t.Fatalf("pause consumer: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/streams/ORDERS/consumers/orders-audit/resume", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"resumed"`) {
		t.Fatalf("resume consumer: %d %s", res.StatusCode, body)
	}

	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/streams/ORDERS/consumers/orders-audit", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete consumer: %d", res.StatusCode)
	}

	// Test Key-Value Store (Create Bucket, Put Key, Get Key, History, Delete Key, Delete Bucket)
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/kv", `{"bucket":"settings","history":3}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"created"`) {
		t.Fatalf("create kv bucket: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/kv", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"settings"`) {
		t.Fatalf("list kv buckets: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/kv/settings/keys/theme", `{"value":"dark"}`)
	if res.StatusCode != 200 || !strings.Contains(body, `"revision"`) {
		t.Fatalf("put kv key: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/kv/settings/keys/theme", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"dark"`) {
		t.Fatalf("get kv key: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/kv/settings/keys/theme/history", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"dark"`) {
		t.Fatalf("get kv history: %d %s", res.StatusCode, body)
	}

	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/kv/settings/keys/theme", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete kv key: %d", res.StatusCode)
	}

	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/kv/settings", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete kv bucket: %d", res.StatusCode)
	}

	// Test Object Store (Create Bucket, Put Object, Get Object, List Objects, Delete Object, Delete Bucket)
	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/objects", `{"bucket":"artifacts","description":"test"}`)
	if res.StatusCode != 201 || !strings.Contains(body, `"created"`) {
		t.Fatalf("create object bucket: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/objects", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"artifacts"`) {
		t.Fatalf("list object buckets: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "POST", ts.URL+"/api/connections/local-dev/objects/artifacts/put?name=test.txt", "hello object store")
	if res.StatusCode != 200 || !strings.Contains(body, `"test.txt"`) {
		t.Fatalf("put object: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/objects/artifacts/get?name=test.txt", "")
	if res.StatusCode != 200 || body != "hello object store" {
		t.Fatalf("get object: %d %s", res.StatusCode, body)
	}

	res, body = do(t, "GET", ts.URL+"/api/connections/local-dev/objects/artifacts/list", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"test.txt"`) {
		t.Fatalf("list objects: %d %s", res.StatusCode, body)
	}

	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/objects/artifacts/delete?name=test.txt", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete object: %d", res.StatusCode)
	}

	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev/objects/artifacts", "")
	if res.StatusCode != 204 {
		t.Fatalf("delete object bucket: %d", res.StatusCode)
	}

	// Remove the connection.
	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev", "")
	if res.StatusCode != 204 {
		t.Errorf("delete: got %d, want 204", res.StatusCode)
	}
	res, _ = do(t, "DELETE", ts.URL+"/api/connections/local-dev", "")
	if res.StatusCode != 404 {
		t.Errorf("second delete: got %d, want 404", res.StatusCode)
	}
}

func TestAuthServer(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := conn.NewManager(log)
	t.Cleanup(mgr.Close)

	ui := fstest.MapFS{"index.html": {Data: []byte("<html>natscope</html>")}}
	authCfg := api.AuthConfig{
		Enabled:  true,
		Username: "admin",
		Password: "secret-password",
	}
	ts := httptest.NewServer(api.New(mgr, ui, "test", log, authCfg))
	t.Cleanup(ts.Close)

	// Health and AuthMe are open
	res, _ := do(t, "GET", ts.URL+"/api/health", "")
	if res.StatusCode != 200 {
		t.Fatalf("health: expected 200, got %d", res.StatusCode)
	}
	res, body := do(t, "GET", ts.URL+"/api/auth/me", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"authenticated":false`) {
		t.Fatalf("auth/me: expected unauthenticated, got %d %s", res.StatusCode, body)
	}

	// Protected endpoint without credentials -> 401
	res, body = do(t, "GET", ts.URL+"/api/connections", "")
	if res.StatusCode != 401 {
		t.Fatalf("expected 401 unauthorized, got %d %s", res.StatusCode, body)
	}

	// Invalid login credentials -> 401
	res, _ = do(t, "POST", ts.URL+"/api/auth/login", `{"username":"admin","password":"wrong"}`)
	if res.StatusCode != 401 {
		t.Fatalf("expected 401 on bad credentials, got %d", res.StatusCode)
	}

	// Successful login -> sets cookie
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret-password"}`))
	req.Header.Set("Content-Type", "application/json")
	loginRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != 200 {
		t.Fatalf("expected 200 login, got %d", loginRes.StatusCode)
	}
	cookies := loginRes.Cookies()
	if len(cookies) == 0 || cookies[0].Name != "natsradar_session" {
		t.Fatalf("missing session cookie")
	}

	// Access protected endpoint with cookie
	connReq, _ := http.NewRequest("GET", ts.URL+"/api/connections", nil)
	connReq.AddCookie(cookies[0])
	connRes, err := http.DefaultClient.Do(connReq)
	if err != nil {
		t.Fatal(err)
	}
	defer connRes.Body.Close()
	if connRes.StatusCode != 200 {
		t.Fatalf("expected 200 with session cookie, got %d", connRes.StatusCode)
	}

	// Also verify HTTP Basic Auth works directly
	basicReq, _ := http.NewRequest("GET", ts.URL+"/api/connections", nil)
	basicReq.SetBasicAuth("admin", "secret-password")
	basicRes, err := http.DefaultClient.Do(basicReq)
	if err != nil {
		t.Fatal(err)
	}
	defer basicRes.Body.Close()
	if basicRes.StatusCode != 200 {
		t.Fatalf("expected 200 with basic auth, got %d", basicRes.StatusCode)
	}
}
