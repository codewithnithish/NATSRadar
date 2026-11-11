// Package testutil provides helpers for tests that need a real NATS server.
package testutil

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// RunNATS starts an in-process NATS server with JetStream enabled on a random
// port and returns its client URL. The server is stopped when the test ends.
func RunNATS(t testing.TB) string {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random free port
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("create nats server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Fatal("nats server did not become ready")
	}
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	return s.ClientURL()
}
