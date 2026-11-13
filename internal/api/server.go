// Package api exposes the NATSRadar REST API and serves the embedded web UI.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/codewithnithish/natsradar/internal/conn"
	"github.com/codewithnithish/natsradar/internal/jsx"
)

const (
	maxBodyBytes   = 1 << 20 // 1 MiB
	requestTimeout = 10 * time.Second
)

// AuthConfig configures UI/API credentials.
type AuthConfig struct {
	Enabled  bool
	Username string
	Password string
}

// Server holds the API dependencies.
type Server struct {
	mgr      *conn.Manager
	log      *slog.Logger
	version  string
	auth     AuthConfig
	sessions sync.Map // token -> username
}

// New builds the HTTP handler: the JSON API under /api and the web UI at /.
// ui may be nil, in which case only the API is served.
func New(mgr *conn.Manager, ui fs.FS, version string, log *slog.Logger, auth ...AuthConfig) http.Handler {
	s := &Server{mgr: mgr, log: log, version: version}
	if len(auth) > 0 {
		s.auth = auth[0]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/auth/me", s.authMe)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)

	mux.HandleFunc("GET /api/connections", s.listConnections)
	mux.HandleFunc("POST /api/connections", s.addConnection)
	mux.HandleFunc("DELETE /api/connections/{id}", s.removeConnection)

	mux.HandleFunc("GET /api/connections/{id}/streams", s.listStreams)
	mux.HandleFunc("POST /api/connections/{id}/streams", s.createStream)
	mux.HandleFunc("POST /api/connections/{id}/streams/{stream}/purge", s.purgeStream)
	mux.HandleFunc("DELETE /api/connections/{id}/streams/{stream}", s.deleteStream)

	mux.HandleFunc("GET /api/connections/{id}/streams/{stream}/consumers", s.listConsumers)
	mux.HandleFunc("POST /api/connections/{id}/streams/{stream}/consumers", s.createConsumer)
	mux.HandleFunc("DELETE /api/connections/{id}/streams/{stream}/consumers/{consumer}", s.deleteConsumer)
	mux.HandleFunc("POST /api/connections/{id}/streams/{stream}/consumers/{consumer}/pause", s.pauseConsumer)
	mux.HandleFunc("POST /api/connections/{id}/streams/{stream}/consumers/{consumer}/resume", s.resumeConsumer)

	mux.HandleFunc("GET /api/connections/{id}/streams/{stream}/messages", s.getMessages)
	mux.HandleFunc("GET /api/connections/{id}/tail", s.tailMessages)
	mux.HandleFunc("POST /api/connections/{id}/publish", s.publishMessage)
	mux.HandleFunc("POST /api/connections/{id}/request", s.sendRequest)
	mux.HandleFunc("GET /api/connections/{id}/clients", s.listClients)
	mux.HandleFunc("GET /api/connections/{id}/leafs", s.listLeafNodes)
	mux.HandleFunc("GET /api/connections/{id}/server", s.getServerDetails)

	// Key-Value Store
	mux.HandleFunc("GET /api/connections/{id}/kv", s.listKVBuckets)
	mux.HandleFunc("POST /api/connections/{id}/kv", s.createKVBucket)
	mux.HandleFunc("DELETE /api/connections/{id}/kv/{bucket}", s.deleteKVBucket)
	mux.HandleFunc("GET /api/connections/{id}/kv/{bucket}/keys", s.listKVKeys)
	mux.HandleFunc("GET /api/connections/{id}/kv/{bucket}/keys/{key}", s.getKVKey)
	mux.HandleFunc("POST /api/connections/{id}/kv/{bucket}/keys/{key}", s.putKVKey)
	mux.HandleFunc("DELETE /api/connections/{id}/kv/{bucket}/keys/{key}", s.deleteKVKey)
	mux.HandleFunc("GET /api/connections/{id}/kv/{bucket}/keys/{key}/history", s.getKVHistory)

	// Object Store
	mux.HandleFunc("GET /api/connections/{id}/objects", s.listObjectBuckets)
	mux.HandleFunc("POST /api/connections/{id}/objects", s.createObjectBucket)
	mux.HandleFunc("DELETE /api/connections/{id}/objects/{bucket}", s.deleteObjectBucket)
	mux.HandleFunc("GET /api/connections/{id}/objects/{bucket}/list", s.listObjects)
	mux.HandleFunc("GET /api/connections/{id}/objects/{bucket}/get", s.getObject)
	mux.HandleFunc("POST /api/connections/{id}/objects/{bucket}/put", s.putObject)
	mux.HandleFunc("DELETE /api/connections/{id}/objects/{bucket}/delete", s.deleteObject)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	if ui != nil {
		mux.Handle("/", spaHandler(ui))
	}
	return s.withMiddleware(mux)
}

// ---- handlers ----

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

func (s *Server) listConnections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type addConnectionRequest struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ReadOnly           bool   `json:"readOnly"`
	Token              string `json:"token"`
	User               string `json:"user"`
	Password           string `json:"password"`
	NKey               string `json:"nkey"`
	Credentials        string `json:"credentials"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	CACert             string `json:"caCert"`
	ClientCert         string `json:"clientCert"`
	ClientKey          string `json:"clientKey"`
	ServerName         string `json:"serverName"`
}

func (s *Server) addConnection(w http.ResponseWriter, r *http.Request) {
	var req addConnectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	entry, err := s.mgr.Add(conn.Profile{
		ID:                 req.ID,
		Name:               req.Name,
		URL:                req.URL,
		ReadOnly:           req.ReadOnly,
		Token:              req.Token,
		User:               req.User,
		Password:           req.Password,
		NKey:               req.NKey,
		Credentials:        req.Credentials,
		InsecureSkipVerify: req.InsecureSkipVerify,
		CACert:             req.CACert,
		ClientCert:         req.ClientCert,
		ClientKey:          req.ClientKey,
		ServerName:         req.ServerName,
	})
	switch {
	case errors.Is(err, conn.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_connection", err.Error())
		return
	case errors.Is(err, conn.ErrExists):
		writeError(w, http.StatusConflict, "connection_exists", err.Error())
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "connect_failed", err.Error())
		return
	}
	for _, info := range s.mgr.List() {
		if info.ID == entry.Profile.ID {
			writeJSON(w, http.StatusCreated, info)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) removeConnection(w http.ResponseWriter, r *http.Request) {
	if !s.mgr.Remove(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "connection_not_found", "unknown connection")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listStreams(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	streams, err := jsx.New(entry.JS).ListStreams(ctx)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, streams)
}

func (s *Server) createStream(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req jsx.CreateStreamConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	summary, err := jsx.New(entry.JS).CreateStream(ctx, req)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, summary)
}

func (s *Server) purgeStream(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := jsx.New(entry.JS).PurgeStream(ctx, r.PathValue("stream"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "purged", "stream": r.PathValue("stream")})
}

func (s *Server) deleteStream(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := jsx.New(entry.JS).DeleteStream(ctx, r.PathValue("stream"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listConsumers(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	consumers, err := jsx.New(entry.JS).ListConsumers(ctx, r.PathValue("stream"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, consumers)
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	startSeq, _ := strconv.ParseUint(r.URL.Query().Get("startSeq"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	subject := r.URL.Query().Get("subject")
	search := r.URL.Query().Get("search")
	var startTime *time.Time
	if st := r.URL.Query().Get("startTime"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = &t
		}
	}

	messages, err := jsx.New(entry.JS).GetMessages(ctx, r.PathValue("stream"), jsx.GetMessagesOptions{
		StartSeq:      startSeq,
		StartTime:     startTime,
		FilterSubject: subject,
		Search:        search,
		Limit:         limit,
	})
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) tailMessages(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusBadRequest, "streaming_unsupported", "streaming is not supported")
		return
	}

	subject := strings.TrimSpace(r.URL.Query().Get("subject"))
	if subject == "" {
		subject = ">"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	msgCh := make(chan *nats.Msg, 128)
	sub, err := entry.NC.Subscribe(subject, func(m *nats.Msg) {
		select {
		case msgCh <- m:
		default:
		}
	})
	if err != nil {
		s.log.Error("tail subscribe failed", "error", err, "subject", subject)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer sub.Unsubscribe()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case m := <-msgCh:
			var seq uint64
			if meta, err := m.Metadata(); err == nil && meta != nil {
				seq = meta.Sequence.Stream
			}

			data := string(m.Data)
			isBinary := !utf8.Valid(m.Data)
			if isBinary {
				data = base64.StdEncoding.EncodeToString(m.Data)
			}

			headers := make(map[string]string)
			for k, v := range m.Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			var h map[string]string
			if len(headers) > 0 {
				h = headers
			}

			msgSummary := jsx.MessageSummary{
				Seq:       seq,
				Subject:   m.Subject,
				Data:      data,
				IsBinary:  isBinary,
				Headers:   h,
				Size:      len(m.Data),
				Timestamp: time.Now().UTC(),
			}

			payload, err := json.Marshal(msgSummary)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

type publishRequest struct {
	Subject string            `json:"subject"`
	Data    string            `json:"data"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (s *Server) publishMessage(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}

	var req publishRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	if req.Subject == "" {
		writeError(w, http.StatusBadRequest, "invalid_subject", "subject is required")
		return
	}

	msg := nats.NewMsg(req.Subject)
	msg.Data = []byte(req.Data)
	for k, v := range req.Headers {
		msg.Header.Set(k, v)
	}

	if err := entry.NC.PublishMsg(msg); err != nil {
		s.log.Error("publish failed", "error", err, "subject", req.Subject)
		writeError(w, http.StatusBadGateway, "publish_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "published",
		"subject": req.Subject,
		"bytes":   len(msg.Data),
	})
}

type requestRequest struct {
	Subject   string            `json:"subject"`
	Data      string            `json:"data"`
	Headers   map[string]string `json:"headers,omitempty"`
	TimeoutMs int               `json:"timeoutMs,omitempty"`
}

type requestResponse struct {
	Subject     string            `json:"subject"`
	Data        string            `json:"data"`
	IsBinary    bool              `json:"isBinary"`
	Headers     map[string]string `json:"headers,omitempty"`
	Size        int               `json:"size"`
	RoundTripMs int64             `json:"roundTripMs"`
}

func (s *Server) sendRequest(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}

	var req requestRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	if req.Subject == "" {
		writeError(w, http.StatusBadRequest, "invalid_subject", "subject is required")
		return
	}

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 10*time.Second {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	msg := nats.NewMsg(req.Subject)
	msg.Data = []byte(req.Data)
	for k, v := range req.Headers {
		msg.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := entry.NC.RequestMsgWithContext(ctx, msg)
	roundTrip := time.Since(start).Milliseconds()

	if err != nil {
		switch {
		case errors.Is(err, nats.ErrNoResponders):
			writeError(w, http.StatusBadGateway, "no_responders", "no service is listening on this subject")
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, nats.ErrTimeout):
			writeError(w, http.StatusGatewayTimeout, "request_timeout", "no response received within timeout")
		default:
			s.log.Error("request failed", "error", err, "subject", req.Subject)
			writeError(w, http.StatusBadGateway, "request_failed", err.Error())
		}
		return
	}

	data := string(resp.Data)
	isBinary := !utf8.Valid(resp.Data)
	if isBinary {
		data = base64.StdEncoding.EncodeToString(resp.Data)
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	var h map[string]string
	if len(headers) > 0 {
		h = headers
	}

	writeJSON(w, http.StatusOK, requestResponse{
		Subject:     resp.Subject,
		Data:        data,
		IsBinary:    isBinary,
		Headers:     h,
		Size:        len(resp.Data),
		RoundTripMs: roundTrip,
	})
}

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	subject := r.URL.Query().Get("subject")
	clients, err := entry.ListClients(ctx, subject)
	if err != nil {
		s.log.Error("list clients failed", "error", err)
		writeError(w, http.StatusBadGateway, "clients_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, clients)
}

func (s *Server) listLeafNodes(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	leafs, err := entry.ListLeafNodes(ctx)
	if err != nil {
		s.log.Error("list leaf nodes failed", "error", err)
		writeError(w, http.StatusBadGateway, "leafs_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, leafs)
}

func (s *Server) getServerDetails(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	details, err := entry.GetServerDetails(ctx)
	if err != nil {
		s.log.Error("get server details failed", "error", err)
		writeError(w, http.StatusBadGateway, "server_details_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, details)
}

// ---- helpers ----

func (s *Server) entry(w http.ResponseWriter, r *http.Request) (*conn.Entry, bool) {
	e, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "connection_not_found", "unknown connection")
	}
	return e, ok
}

// jetstreamError maps client errors to stable API error codes.
func (s *Server) jetstreamError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, jetstream.ErrJetStreamNotEnabled),
		errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount),
		errors.Is(err, nats.ErrNoResponders):
		writeError(w, http.StatusConflict, "jetstream_not_enabled",
			"JetStream is not enabled on this server or account (start nats-server with -js)")
	case errors.Is(err, jetstream.ErrStreamNotFound):
		writeError(w, http.StatusNotFound, "stream_not_found", "unknown stream")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, nats.ErrTimeout):
		writeError(w, http.StatusGatewayTimeout, "nats_timeout", "NATS did not answer in time")
	case errors.Is(err, nats.ErrConnectionClosed), errors.Is(err, nats.ErrDisconnected):
		writeError(w, http.StatusServiceUnavailable, "nats_unavailable", "not connected to NATS")
	default:
		s.log.Error("jetstream request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var b errorBody
	b.Error.Code = code
	b.Error.Message = message
	writeJSON(w, status, b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// ---- web UI ----

// spaHandler serves static files and falls back to index.html so client-side
// routes work on refresh.
func spaHandler(ui fs.FS) http.Handler {
	files := http.FileServer(http.FS(ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if st, err := fs.Stat(ui, p); err == nil && !st.IsDir() {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		index, err := fs.ReadFile(ui, "index.html")
		if err != nil {
			http.Error(w, "Web UI is not built. Run `make web` (or `make dev-web` for development).",
				http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

// ---- auth handlers ----

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.auth.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.auth.Password)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "incorrect username or password")
		return
	}
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	token := hex.EncodeToString(buf)
	s.sessions.Store(token, req.Username)

	http.SetCookie(w, &http.Cookie{
		Name:     "natsradar_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"username": req.Username,
		"token":    token,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("natsradar_session"); err == nil {
		s.sessions.Delete(c.Value)
	}
	if c, err := r.Cookie("natscope_session"); err == nil {
		s.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "natsradar_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "natscope_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	if !s.auth.Enabled {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"authEnabled":   false,
			"authenticated": true,
			"username":      "anonymous",
		})
		return
	}
	auth := s.isAuthenticated(r)
	u := ""
	if auth {
		u = s.auth.Username
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authEnabled":   true,
		"authenticated": auth,
		"username":      u,
	})
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	if !s.auth.Enabled {
		return true
	}
	// Basic Auth
	if u, p, ok := r.BasicAuth(); ok {
		if subtle.ConstantTimeCompare([]byte(u), []byte(s.auth.Username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(p), []byte(s.auth.Password)) == 1 {
			return true
		}
	}
	// Cookie or Bearer token
	token := ""
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
		token = strings.TrimPrefix(ah, "Bearer ")
	}
	if token == "" {
		if c, err := r.Cookie("natsradar_session"); err == nil {
			token = c.Value
		} else if c, err := r.Cookie("natscope_session"); err == nil {
			token = c.Value
		}
	}
	if token != "" {
		if _, ok := s.sessions.Load(token); ok {
			return true
		}
	}
	return false
}

// ---- consumer management handlers ----

type createConsumerRequest struct {
	Name          string `json:"name"`
	Durable       string `json:"durable"`
	Description   string `json:"description"`
	FilterSubject string `json:"filterSubject"`
	DeliverPolicy string `json:"deliverPolicy"`
	AckPolicy     string `json:"ackPolicy"`
	MaxAckPending int    `json:"maxAckPending"`
	MaxDeliver    int    `json:"maxDeliver"`
}

func (s *Server) createConsumer(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req createConsumerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg := jetstream.ConsumerConfig{
		Name:          strings.TrimSpace(req.Name),
		Durable:       strings.TrimSpace(req.Durable),
		Description:   strings.TrimSpace(req.Description),
		FilterSubject: strings.TrimSpace(req.FilterSubject),
		MaxAckPending: req.MaxAckPending,
		MaxDeliver:    req.MaxDeliver,
	}
	if cfg.Durable == "" && cfg.Name != "" {
		cfg.Durable = cfg.Name
	}
	switch strings.ToLower(req.DeliverPolicy) {
	case "all":
		cfg.DeliverPolicy = jetstream.DeliverAllPolicy
	case "last":
		cfg.DeliverPolicy = jetstream.DeliverLastPolicy
	case "new":
		cfg.DeliverPolicy = jetstream.DeliverNewPolicy
	case "last_per_subject":
		cfg.DeliverPolicy = jetstream.DeliverLastPerSubjectPolicy
	default:
		cfg.DeliverPolicy = jetstream.DeliverAllPolicy
	}
	switch strings.ToLower(req.AckPolicy) {
	case "none":
		cfg.AckPolicy = jetstream.AckNonePolicy
	case "all":
		cfg.AckPolicy = jetstream.AckAllPolicy
	default:
		cfg.AckPolicy = jetstream.AckExplicitPolicy
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	consumer, err := jsx.New(entry.JS).CreateConsumer(ctx, r.PathValue("stream"), cfg)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, consumer)
}

func (s *Server) deleteConsumer(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := jsx.New(entry.JS).DeleteConsumer(ctx, r.PathValue("stream"), r.PathValue("consumer"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pauseConsumerRequest struct {
	PauseMinutes int `json:"pauseMinutes"`
}

func (s *Server) pauseConsumer(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req pauseConsumerRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	mins := req.PauseMinutes
	if mins <= 0 {
		mins = 60
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := jsx.New(entry.JS).PauseConsumer(ctx, r.PathValue("stream"), r.PathValue("consumer"), time.Now().Add(time.Duration(mins)*time.Minute))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused", "consumer": r.PathValue("consumer")})
}

func (s *Server) resumeConsumer(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := jsx.New(entry.JS).ResumeConsumer(ctx, r.PathValue("stream"), r.PathValue("consumer"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed", "consumer": r.PathValue("consumer")})
}

// ---- Key-Value handlers ----

type createKVBucketRequest struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description"`
	History     int    `json:"history"`
	TTLSeconds  int    `json:"ttlSeconds"`
}

func (s *Server) listKVBuckets(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	buckets, err := jsx.New(entry.JS).ListKVBuckets(ctx)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buckets)
}

func (s *Server) createKVBucket(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req createKVBucketRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	bucket := strings.TrimSpace(req.Bucket)
	if bucket == "" {
		writeError(w, http.StatusBadRequest, "invalid_bucket", "bucket name is required")
		return
	}
	cfg := jetstream.KeyValueConfig{
		Bucket:      bucket,
		Description: req.Description,
		History:     uint8(req.History),
		TTL:         time.Duration(req.TTLSeconds) * time.Second,
	}
	if cfg.History == 0 {
		cfg.History = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := jsx.New(entry.JS).CreateKVBucket(ctx, cfg); err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "bucket": bucket})
}

func (s *Server) deleteKVBucket(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := jsx.New(entry.JS).DeleteKVBucket(ctx, r.PathValue("bucket")); err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listKVKeys(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	keys, err := jsx.New(entry.JS).ListKVKeys(ctx, r.PathValue("bucket"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) getKVKey(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	val, err := jsx.New(entry.JS).GetKVKey(ctx, r.PathValue("bucket"), r.PathValue("key"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, val)
}

type putKVKeyRequest struct {
	Value string `json:"value"`
}

func (s *Server) putKVKey(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req putKVKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	rev, err := jsx.New(entry.JS).PutKVKey(ctx, r.PathValue("bucket"), r.PathValue("key"), []byte(req.Value))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "revision": rev})
}

func (s *Server) deleteKVKey(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := jsx.New(entry.JS).DeleteKVKey(ctx, r.PathValue("bucket"), r.PathValue("key")); err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getKVHistory(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	hist, err := jsx.New(entry.JS).GetKVHistory(ctx, r.PathValue("bucket"), r.PathValue("key"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hist)
}

// ---- Object Store handlers ----

type createObjectBucketRequest struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description"`
	TTL         string `json:"ttl,omitempty"`      // e.g. "24h", "7d", "30d", "1h"
	MaxBytes    int64  `json:"maxBytes,omitempty"` // Maximum storage capacity in bytes
	Storage     string `json:"storage,omitempty"`  // "file" (default) or "memory"
	Replicas    int    `json:"replicas,omitempty"` // Replication factor (default 1)
}

func (s *Server) listObjectBuckets(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	buckets, err := jsx.New(entry.JS).ListObjectBuckets(ctx)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buckets)
}

func (s *Server) createObjectBucket(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	var req createObjectBucketRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	bucket := strings.TrimSpace(req.Bucket)
	if bucket == "" {
		writeError(w, http.StatusBadRequest, "invalid_bucket", "bucket name is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	var ttl time.Duration
	if req.TTL != "" {
		if d, err := time.ParseDuration(req.TTL); err == nil {
			ttl = d
		} else if n, err := strconv.ParseInt(req.TTL, 10, 64); err == nil {
			ttl = time.Duration(n) * time.Second
		}
	}
	storageType := jetstream.FileStorage
	if strings.ToLower(strings.TrimSpace(req.Storage)) == "memory" {
		storageType = jetstream.MemoryStorage
	}
	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	if err := jsx.New(entry.JS).CreateObjectBucket(ctx, jetstream.ObjectStoreConfig{
		Bucket:      bucket,
		Description: req.Description,
		TTL:         ttl,
		MaxBytes:    req.MaxBytes,
		Storage:     storageType,
		Replicas:    replicas,
	}); err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "bucket": bucket})
}

func (s *Server) deleteObjectBucket(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := jsx.New(entry.JS).DeleteObjectBucket(ctx, r.PathValue("bucket")); err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	objects, err := jsx.New(entry.JS).ListObjects(ctx, r.PathValue("bucket"))
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, objects)
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "object name is required in query parameter ?name=...")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	data, info, err := jsx.New(entry.JS).GetObject(ctx, r.PathValue("bucket"), name)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", path.Base(info.Name)))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "object name is required in query parameter ?name=...")
		return
	}
	// Limit to 20MB for direct upload
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 20<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_error", "failed to read object body: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	info, err := jsx.New(entry.JS).PutObject(ctx, r.PathValue("bucket"), name, data)
	if err != nil {
		s.jetstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"name":   info.Name,
		"size":   info.Size,
		"digest": info.Digest,
	})
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.entry(w, r)
	if !ok {
		return
	}
	if entry.Profile.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "connection is configured as read-only")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "object name is required in query parameter ?name=...")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if err := jsx.New(entry.JS).DeleteObject(ctx, r.PathValue("bucket"), name); err != nil {
		s.jetstreamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- middleware ----

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		h := rec.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic in handler", "panic", v, "path", r.URL.Path)
				writeError(rec, http.StatusInternalServerError, "internal", "internal error")
			}
			s.log.Debug("request", "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "duration", time.Since(start))
		}()

		// Authentication check for API endpoints
		if s.auth.Enabled && strings.HasPrefix(r.URL.Path, "/api/") {
			if r.URL.Path != "/api/health" && r.URL.Path != "/api/auth/login" && r.URL.Path != "/api/auth/me" {
				if !s.isAuthenticated(r) {
					writeError(rec, http.StatusUnauthorized, "unauthorized", "authentication required")
					return
				}
			}
		}

		next.ServeHTTP(rec, r)
	})
}
