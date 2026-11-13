// Package conn manages the NATS connections Natscope is pointed at.
package conn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/codewithnithish/natscope/internal/storage"
)

var (
	// ErrExists is returned when a connection id is already in use.
	ErrExists = errors.New("connection already exists")
	// ErrInvalid is returned for an invalid connection profile.
	ErrInvalid = errors.New("invalid connection profile")

	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	slugCleaner = regexp.MustCompile(`[^a-z0-9]+`)
)

// Profile describes how to reach a NATS server.
type Profile struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ReadOnly           bool   `json:"readOnly"`
	Token              string `json:"token,omitempty"`
	User               string `json:"user,omitempty"`
	Password           string `json:"password,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
	CACert             string `json:"caCert,omitempty"`
	ClientCert         string `json:"clientCert,omitempty"`
	ClientKey          string `json:"clientKey,omitempty"`
	ServerName         string `json:"serverName,omitempty"`
}

// Entry is a live connection plus its JetStream handle.
type Entry struct {
	Profile Profile
	NC      *nats.Conn
	JS      jetstream.JetStream
}

// Info is the public, secret-free view of a connection.
type Info struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ReadOnly           bool   `json:"readOnly"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"`
	HasTLS             bool   `json:"hasTls,omitempty"`
	Status             string `json:"status"`
	ServerVersion      string `json:"serverVersion,omitempty"`
	ConnectedURL       string `json:"connectedUrl,omitempty"`
}

// Manager owns all connections. It is safe for concurrent use.
type Manager struct {
	log         *slog.Logger
	storagePath string
	driver      storage.Driver
	mu          sync.RWMutex
	entries     map[string]*Entry
}

// NewManager creates a Manager, optionally configured to persist to storagePath.
func NewManager(log *slog.Logger, storagePath ...string) *Manager {
	m := &Manager{log: log, entries: make(map[string]*Entry)}
	if len(storagePath) > 0 {
		m.storagePath = storagePath[0]
	}
	return m
}

// SetStorageDriver sets an explicit pluggable storage driver (Local, S3, NFS, etc.).
func (m *Manager) SetStorageDriver(d storage.Driver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.driver = d
}

// Load restores persisted connection profiles from storage.
func (m *Manager) Load() error {
	var data []byte
	var err error

	if m.driver != nil {
		data, err = m.driver.Load(context.Background(), m.storagePath)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil
			}
			return err
		}
	} else if m.storagePath != "" {
		data, err = os.ReadFile(m.storagePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
	} else {
		return nil
	}

	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return err
	}
	for _, p := range profiles {
		if _, err := m.Add(p); err != nil {
			m.log.Warn("failed to restore connection profile", "id", p.ID, "error", err)
		}
	}
	return nil
}

func (m *Manager) saveLocked() {
	if m.storagePath == "" && m.driver == nil {
		return
	}
	profiles := make([]Profile, 0, len(m.entries))
	for _, e := range m.entries {
		profiles = append(profiles, e.Profile)
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		m.log.Warn("failed to encode connections for persistence", "error", err)
		return
	}

	if m.driver != nil {
		if err := m.driver.Save(context.Background(), m.storagePath, data); err != nil {
			m.log.Warn("failed to persist connections via driver", "driver", m.driver.Name(), "error", err)
		}
		return
	}

	dir := filepath.Dir(m.storagePath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	if err := os.WriteFile(m.storagePath, data, 0600); err != nil {
		m.log.Warn("failed to persist connections", "path", m.storagePath, "error", err)
	}
}

// Add validates the profile, connects (retrying in the background if the server
// is not reachable yet) and registers it.
func (m *Manager) Add(p Profile) (*Entry, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.URL = strings.TrimSpace(p.URL)
	if p.URL == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalid)
	}
	if p.Name == "" {
		p.Name = p.URL
	}
	if p.ID == "" {
		p.ID = slugify(p.Name)
	}
	if !idPattern.MatchString(p.ID) {
		return nil, fmt.Errorf("%w: id must match %s", ErrInvalid, idPattern)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[p.ID]; ok {
		return nil, fmt.Errorf("%w: %s", ErrExists, p.ID)
	}

	id := p.ID
	opts := []nats.Option{
		nats.Name("natscope"),
		nats.Timeout(5 * time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		// Do not fail if the server is down at startup; keep retrying so the UI
		// can show "RECONNECTING" instead of refusing to start.
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			m.log.Warn("nats disconnected", "connection", id, "error", err)
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			m.log.Info("nats reconnected", "connection", id, "server", c.ConnectedUrlRedacted())
		}),
	}
	switch {
	case p.Token != "":
		opts = append(opts, nats.Token(p.Token))
	case p.User != "":
		opts = append(opts, nats.UserInfo(p.User, p.Password))
	}

	p.CACert = strings.TrimSpace(p.CACert)
	p.ClientCert = strings.TrimSpace(p.ClientCert)
	p.ClientKey = strings.TrimSpace(p.ClientKey)
	p.ServerName = strings.TrimSpace(p.ServerName)

	hasTLS := p.InsecureSkipVerify || p.CACert != "" || p.ClientCert != "" || p.ServerName != "" ||
		strings.HasPrefix(p.URL, "tls://") || strings.HasPrefix(p.URL, "wss://")

	if hasTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.InsecureSkipVerify,
			ServerName:         p.ServerName,
		}
		if p.CACert != "" {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(p.CACert)) {
				return nil, fmt.Errorf("%w: failed to parse ca certificate", ErrInvalid)
			}
			tlsConfig.RootCAs = pool
		}
		if p.ClientCert != "" && p.ClientKey != "" {
			cert, err := tls.X509KeyPair([]byte(p.ClientCert), []byte(p.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("%w: invalid client cert/key: %v", ErrInvalid, err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	nc, err := nats.Connect(p.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", p.URL, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	e := &Entry{Profile: p, NC: nc, JS: js}
	m.entries[p.ID] = e
	m.saveLocked()
	m.log.Info("connection added", "connection", p.ID, "url", p.URL, "readOnly", p.ReadOnly)
	return e, nil
}

// Get returns the connection with the given id.
func (m *Manager) Get(id string) (*Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[id]
	return e, ok
}

// Remove closes and forgets a connection. It reports whether it existed.
func (m *Manager) Remove(id string) bool {
	m.mu.Lock()
	e, ok := m.entries[id]
	if ok {
		delete(m.entries, id)
		m.saveLocked()
	}
	m.mu.Unlock()
	if ok {
		e.NC.Close()
		m.log.Info("connection removed", "connection", id)
	}
	return ok
}

// List returns all connections sorted by id.
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Info, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e.info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Close closes every connection.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.entries {
		e.NC.Close()
		delete(m.entries, id)
	}
}

func (e *Entry) info() Info {
	hasTLS := e.Profile.InsecureSkipVerify || e.Profile.CACert != "" || e.Profile.ClientCert != "" ||
		strings.HasPrefix(e.Profile.URL, "tls://") || strings.HasPrefix(e.Profile.URL, "wss://")
	return Info{
		ID:                 e.Profile.ID,
		Name:               e.Profile.Name,
		URL:                e.Profile.URL,
		ReadOnly:           e.Profile.ReadOnly,
		InsecureSkipVerify: e.Profile.InsecureSkipVerify,
		HasTLS:             hasTLS,
		Status:             e.NC.Status().String(),
		ServerVersion:      e.NC.ConnectedServerVersion(),
		ConnectedURL:       e.NC.ConnectedUrlRedacted(),
	}
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugCleaner.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = strings.Trim(s[:63], "-")
	}
	return s
}

// ServerDetails holds NATS server runtime stats and JetStream configuration limits.
type ServerDetails struct {
	ServerID         string  `json:"serverId"`
	ServerName       string  `json:"serverName"`
	Version          string  `json:"version"`
	GoVersion        string  `json:"goVersion"`
	Host             string  `json:"host"`
	Port             int     `json:"port"`
	Uptime           string  `json:"uptime"`
	MemBytes         int64   `json:"memBytes"`
	Cores            int     `json:"cores"`
	CPU              float64 `json:"cpu"`
	Connections      int     `json:"connections"`
	TotalConnections uint64  `json:"totalConnections"`
	Subscriptions    uint32  `json:"subscriptions"`
	InMsgs           int64   `json:"inMsgs"`
	OutMsgs          int64   `json:"outMsgs"`
	InBytes          int64   `json:"inBytes"`
	OutBytes         int64   `json:"outBytes"`
	MaxPayload       int     `json:"maxPayload"`
	ClusterName      string  `json:"clusterName,omitempty"`
	LeafNodes        int     `json:"leafNodes"`
	JetStreamEnabled bool    `json:"jetStreamEnabled"`
	JSMaxMemory      int64   `json:"jsMaxMemory,omitempty"`
	JSMaxStorage     int64   `json:"jsMaxStorage,omitempty"`
	JSMemoryUsed     int64   `json:"jsMemoryUsed,omitempty"`
	JSStorageUsed    int64   `json:"jsStorageUsed,omitempty"`
	JSStoreDir       string  `json:"jsStoreDir,omitempty"`
}

type sysVarzResponse struct {
	Data sysVarzData `json:"data"`
	sysVarzData
}

type sysVarzData struct {
	ServerID         string  `json:"server_id"`
	ServerName       string  `json:"server_name"`
	Version          string  `json:"version"`
	GoVersion        string  `json:"go"`
	Host             string  `json:"host"`
	Port             int     `json:"port"`
	Uptime           string  `json:"uptime"`
	Mem              int64   `json:"mem"`
	Cores            int     `json:"cores"`
	CPU              float64 `json:"cpu"`
	Connections      int     `json:"connections"`
	TotalConnections uint64  `json:"total_connections"`
	Subscriptions    uint32  `json:"subscriptions"`
	InMsgs           int64   `json:"in_msgs"`
	OutMsgs          int64   `json:"out_msgs"`
	InBytes          int64   `json:"in_bytes"`
	OutBytes         int64   `json:"out_bytes"`
	MaxPayload       int     `json:"max_payload"`
	LeafNodes        int     `json:"leafnodes"`
	Cluster          struct {
		Name string `json:"name"`
	} `json:"cluster"`
	JetStream struct {
		Config struct {
			MaxMemory  int64  `json:"max_memory"`
			MaxStorage int64  `json:"max_storage"`
			StoreDir   string `json:"store_dir"`
		} `json:"config"`
		Stats struct {
			Memory  int64 `json:"memory"`
			Storage int64 `json:"storage"`
		} `json:"stats"`
	} `json:"jetstream"`
}

// GetServerDetails queries NATS for server runtime information, cluster info, and JetStream limits.
func (e *Entry) GetServerDetails(ctx context.Context) (*ServerDetails, error) {
	var data sysVarzData
	gotVarz := false

	msg, err := e.NC.RequestWithContext(ctx, "$SYS.REQ.SERVER.PING.VARZ", nil)
	if err == nil {
		var resp sysVarzResponse
		if err := json.Unmarshal(msg.Data, &resp); err == nil {
			data = resp.Data
			if data.ServerID == "" && resp.sysVarzData.ServerID != "" {
				data = resp.sysVarzData
			}
			gotVarz = true
		}
	}

	// Fallback to HTTP monitoring port (e.g. port 8222) if $SYS request was unauthorized or unresponded
	if !gotVarz {
		if u, err := url.Parse(e.NC.ConnectedUrl()); err == nil && u.Hostname() != "" {
			monitorURL := fmt.Sprintf("http://%s:8222/varz", u.Hostname())
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, monitorURL, nil)
			if reqErr == nil {
				client := &http.Client{Timeout: 1500 * time.Millisecond}
				if resp, err := client.Do(req); err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						var httpData sysVarzData
						if err := json.NewDecoder(resp.Body).Decode(&httpData); err == nil {
							data = httpData
							gotVarz = true
						}
					}
				}
			}
		}
	}

	serverID := data.ServerID
	if serverID == "" {
		serverID = e.NC.ConnectedServerId()
	}
	serverName := data.ServerName
	if serverName == "" {
		serverName = e.Profile.Name
	}
	version := data.Version
	if version == "" {
		version = e.NC.ConnectedServerVersion()
	}
	host := data.Host
	if host == "" {
		host = e.NC.ConnectedUrl()
	}

	jsEnabled := e.JS != nil
	var jsMaxMem, jsMaxStorage, jsMemUsed, jsStorageUsed int64
	var jsStoreDir string
	if gotVarz {
		jsMaxMem = data.JetStream.Config.MaxMemory
		jsMaxStorage = data.JetStream.Config.MaxStorage
		jsMemUsed = data.JetStream.Stats.Memory
		jsStorageUsed = data.JetStream.Stats.Storage
		jsStoreDir = data.JetStream.Config.StoreDir
		if data.JetStream.Config.MaxStorage > 0 || data.JetStream.Config.StoreDir != "" {
			jsEnabled = true
		}
	}

	// Always augment or backfill JetStream storage & limits from JetStream AccountInfo
	if e.JS != nil {
		if acct, err := e.JS.AccountInfo(ctx); err == nil && acct != nil {
			jsEnabled = true
			if jsStorageUsed == 0 {
				jsStorageUsed = int64(acct.Store)
			}
			if jsMemUsed == 0 {
				jsMemUsed = int64(acct.Memory)
			}
			if jsMaxStorage <= 0 && acct.Limits.MaxStore > 0 {
				jsMaxStorage = acct.Limits.MaxStore
			}
			if jsMaxMem <= 0 && acct.Limits.MaxMemory > 0 {
				jsMaxMem = acct.Limits.MaxMemory
			}
		}
	}

	return &ServerDetails{
		ServerID:         serverID,
		ServerName:       serverName,
		Version:          version,
		GoVersion:        data.GoVersion,
		Host:             host,
		Port:             data.Port,
		Uptime:           data.Uptime,
		MemBytes:         data.Mem,
		Cores:            data.Cores,
		CPU:              data.CPU,
		Connections:      data.Connections,
		TotalConnections: data.TotalConnections,
		Subscriptions:    data.Subscriptions,
		InMsgs:           data.InMsgs,
		OutMsgs:          data.OutMsgs,
		InBytes:          data.InBytes,
		OutBytes:         data.OutBytes,
		MaxPayload:       data.MaxPayload,
		ClusterName:      data.Cluster.Name,
		LeafNodes:        data.LeafNodes,
		JetStreamEnabled: jsEnabled,
		JSMaxMemory:      jsMaxMem,
		JSMaxStorage:     jsMaxStorage,
		JSMemoryUsed:     jsMemUsed,
		JSStorageUsed:    jsStorageUsed,
		JSStoreDir:       jsStoreDir,
	}, nil
}

// ClientInfo describes a connected microservice/client and its subject subscriptions.
type ClientInfo struct {
	CID                   uint64   `json:"cid"`
	Name                  string   `json:"name"`
	Lang                  string   `json:"lang"`
	Version               string   `json:"version"`
	IP                    string   `json:"ip"`
	Port                  int      `json:"port"`
	Uptime                string   `json:"uptime"`
	InMsgs                int64    `json:"inMsgs"`
	OutMsgs               int64    `json:"outMsgs"`
	InBytes               int64    `json:"inBytes"`
	OutBytes              int64    `json:"outBytes"`
	Subscriptions         []string `json:"subscriptions"`
	MatchingSubscriptions []string `json:"matchingSubscriptions,omitempty"`
	IsInternal            bool     `json:"isInternal"`
	Kind                  string   `json:"kind"` // "internal" | "client"
}

type sysConnzResponse struct {
	Data struct {
		Connections []struct {
			CID               uint64   `json:"cid"`
			Name              string   `json:"name"`
			Lang              string   `json:"lang"`
			Version           string   `json:"version"`
			IP                string   `json:"ip"`
			Port              int      `json:"port"`
			Uptime            string   `json:"uptime"`
			InMsgs            int64    `json:"in_msgs"`
			OutMsgs           int64    `json:"out_msgs"`
			InBytes           int64    `json:"in_bytes"`
			OutBytes          int64    `json:"out_bytes"`
			SubscriptionsList []string `json:"subscriptions_list"`
		} `json:"connections"`
	} `json:"data"`
}

// ListClients queries NATS for connected clients and their active subscriptions.
// If subjectFilter is non-empty, only clients with matching subscriptions are returned.
func (e *Entry) ListClients(ctx context.Context, subjectFilter string) ([]ClientInfo, error) {
	msg, err := e.NC.RequestWithContext(ctx, "$SYS.REQ.SERVER.PING.CONNZ", []byte(`{"subscriptions":true}`))
	if err != nil {
		return nil, fmt.Errorf("system connz request: %w", err)
	}

	var resp sysConnzResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("decode connz response: %w", err)
	}

	subjectFilter = strings.TrimSpace(subjectFilter)
	clients := make([]ClientInfo, 0, len(resp.Data.Connections))

	for _, c := range resp.Data.Connections {
		subs := make([]string, 0, len(c.SubscriptionsList))
		matching := make([]string, 0)

		for _, s := range c.SubscriptionsList {
			if strings.HasPrefix(s, "_INBOX.") && !strings.HasPrefix(subjectFilter, "_INBOX.") {
				continue
			}
			subs = append(subs, s)
			if subjectFilter != "" && subjectsOverlap(subjectFilter, s) {
				matching = append(matching, s)
			}
		}

		if subjectFilter != "" && len(matching) == 0 {
			continue
		}

		name := c.Name
		if name == "" {
			name = fmt.Sprintf("%s:%d", c.IP, c.Port)
		}

		isInternal := strings.EqualFold(c.Name, "natscope") || strings.HasPrefix(strings.ToLower(c.Name), "natscope-")
		kind := "client"
		if isInternal {
			kind = "internal"
		}

		clients = append(clients, ClientInfo{
			CID:                   c.CID,
			Name:                  name,
			Lang:                  c.Lang,
			Version:               c.Version,
			IP:                    c.IP,
			Port:                  c.Port,
			Uptime:                c.Uptime,
			InMsgs:                c.InMsgs,
			OutMsgs:               c.OutMsgs,
			InBytes:               c.InBytes,
			OutBytes:              c.OutBytes,
			Subscriptions:         subs,
			MatchingSubscriptions: matching,
			IsInternal:            isInternal,
			Kind:                  kind,
		})
	}

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Name < clients[j].Name
	})

	return clients, nil
}

// LeafNodeInfo describes a connected leaf node edge server.
type LeafNodeInfo struct {
	Account       string `json:"account"`
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Name          string `json:"name"`
	RTT           string `json:"rtt,omitempty"`
	InMsgs        int64  `json:"inMsgs"`
	OutMsgs       int64  `json:"outMsgs"`
	InBytes       int64  `json:"inBytes"`
	OutBytes      int64  `json:"outBytes"`
	Subscriptions uint32 `json:"subscriptions"`
	IsSpoke       bool   `json:"isSpoke"`
}

// LeafNodesSummary is the aggregate overview of all connected edge leaf nodes.
type LeafNodesSummary struct {
	ServerID string         `json:"serverId"`
	Count    int            `json:"count"`
	Leafs    []LeafNodeInfo `json:"leafs"`
}

type sysLeafzResponse struct {
	Data sysLeafzData `json:"data"`
	sysLeafzData
}

type sysLeafzData struct {
	ServerID     string    `json:"server_id"`
	NumLeafnodes int       `json:"num_leafnodes"`
	Leafs        []sysLeaf `json:"leafs"`
}

type sysLeaf struct {
	Account       string `json:"account"`
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Name          string `json:"name"`
	RTT           string `json:"rtt"`
	InMsgs        int64  `json:"in_msgs"`
	OutMsgs       int64  `json:"out_msgs"`
	InBytes       int64  `json:"in_bytes"`
	OutBytes      int64  `json:"out_bytes"`
	Subscriptions uint32 `json:"subscriptions"`
	IsSpoke       bool   `json:"is_spoke"`
}

// ListLeafNodes queries NATS for connected leaf node servers via $SYS or HTTP monitoring fallback.
func (e *Entry) ListLeafNodes(ctx context.Context) (*LeafNodesSummary, error) {
	var data sysLeafzData
	gotData := false

	// 1. Try NATS $SYS request
	if msg, err := e.NC.RequestWithContext(ctx, "$SYS.REQ.SERVER.PING.LEAFZ", []byte("{}")); err == nil {
		var resp sysLeafzResponse
		if err := json.Unmarshal(msg.Data, &resp); err == nil {
			if len(resp.Data.Leafs) > 0 || resp.Data.NumLeafnodes > 0 {
				data = resp.Data
				gotData = true
			} else if len(resp.Leafs) > 0 || resp.NumLeafnodes > 0 {
				data = resp.sysLeafzData
				gotData = true
			}
		}
	}

	// 2. Fallback to HTTP monitoring endpoint /leafz if available
	if !gotData && e.NC.ConnectedUrl() != "" {
		if u, err := url.Parse(e.NC.ConnectedUrl()); err == nil && u.Hostname() != "" {
			monitorURL := fmt.Sprintf("http://%s:8222/leafz", u.Hostname())
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, monitorURL, nil)
			if reqErr == nil {
				client := &http.Client{Timeout: 1500 * time.Millisecond}
				if resp, err := client.Do(req); err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						var httpData sysLeafzData
						if err := json.NewDecoder(resp.Body).Decode(&httpData); err == nil {
							data = httpData
							gotData = true
						}
					}
				}
			}
		}
	}

	serverID := data.ServerID
	if serverID == "" {
		serverID = e.NC.ConnectedServerId()
	}

	result := &LeafNodesSummary{
		ServerID: serverID,
		Count:    len(data.Leafs),
		Leafs:    make([]LeafNodeInfo, 0, len(data.Leafs)),
	}

	for _, l := range data.Leafs {
		name := l.Name
		if name == "" {
			name = fmt.Sprintf("leaf-%s:%d", l.IP, l.Port)
		}
		acct := l.Account
		if acct == "" {
			acct = "$G"
		}
		result.Leafs = append(result.Leafs, LeafNodeInfo{
			Account:       acct,
			IP:            l.IP,
			Port:          l.Port,
			Name:          name,
			RTT:           l.RTT,
			InMsgs:        l.InMsgs,
			OutMsgs:       l.OutMsgs,
			InBytes:       l.InBytes,
			OutBytes:      l.OutBytes,
			Subscriptions: l.Subscriptions,
			IsSpoke:       l.IsSpoke,
		})
	}

	sort.Slice(result.Leafs, func(i, j int) bool {
		return result.Leafs[i].Name < result.Leafs[j].Name
	})

	return result, nil
}

func matchSubject(pattern, subject string) bool {
	if pattern == "" || pattern == ">" || pattern == subject {
		return true
	}
	pTokens := strings.Split(pattern, ".")
	sTokens := strings.Split(subject, ".")

	for i := 0; i < len(pTokens); i++ {
		if pTokens[i] == ">" {
			return true
		}
		if i >= len(sTokens) {
			return false
		}
		if pTokens[i] != "*" && pTokens[i] != sTokens[i] {
			return false
		}
	}
	return len(pTokens) == len(sTokens)
}

func subjectsOverlap(sub1, sub2 string) bool {
	return matchSubject(sub1, sub2) || matchSubject(sub2, sub1)
}
