package jsx_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/codewithnithish/natsradar/internal/jsx"
	"github.com/codewithnithish/natsradar/internal/testutil"
)

func setup(t *testing.T) (jetstream.JetStream, context.Context) {
	t.Helper()
	nc, err := nats.Connect(testutil.RunNATS(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return js, ctx
}

func TestListStreamsRollsUpConsumerLag(t *testing.T) {
	js, ctx := setup(t)

	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name: "DM", Subjects: []string{"dm.>"}, Storage: jetstream.MemoryStorage,
	}); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := js.Publish(ctx, "dm.created", []byte("x")); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	cons, err := js.CreateOrUpdateConsumer(ctx, "DM", jetstream.ConsumerConfig{
		Durable: "dcs-service", FilterSubject: "dm.>", AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	// Deliver 4 messages, ack 2: expect 6 never delivered (pending) and 2 in flight.
	batch, err := cons.Fetch(4, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	n := 0
	for msg := range batch.Messages() {
		if n < 2 {
			if err := msg.DoubleAck(ctx); err != nil {
				t.Fatalf("ack: %v", err)
			}
		}
		n++
	}

	streams, err := jsx.New(js).ListStreams(ctx)
	if err != nil {
		t.Fatalf("ListStreams: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1", len(streams))
	}
	s := streams[0]
	if s.Name != "DM" || s.Messages != 10 || s.ConsumerCount != 1 {
		t.Errorf("unexpected stream summary: %+v", s)
	}
	if s.Pending != 6 || s.AckPending != 2 {
		t.Errorf("lag rollup: pending=%d ackPending=%d, want 6 and 2", s.Pending, s.AckPending)
	}
}

func TestListConsumers(t *testing.T) {
	js, ctx := setup(t)

	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name: "DCS", Subjects: []string{"dcs.>"}, Storage: jetstream.MemoryStorage,
	}); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if _, err := js.CreateOrUpdateConsumer(ctx, "DCS", jetstream.ConsumerConfig{
		Durable: "dm-service", FilterSubject: "dcs.processed",
		AckPolicy: jetstream.AckExplicitPolicy, MaxDeliver: 5, AckWait: 30 * time.Second,
	}); err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	consumers, err := jsx.New(js).ListConsumers(ctx, "DCS")
	if err != nil {
		t.Fatalf("ListConsumers: %v", err)
	}
	if len(consumers) != 1 {
		t.Fatalf("got %d consumers, want 1", len(consumers))
	}
	c := consumers[0]
	if c.Name != "dm-service" || !c.Durable || c.AckPolicy != "explicit" ||
		c.MaxDeliver != 5 || c.AckWaitMs != 30000 {
		t.Errorf("unexpected consumer summary: %+v", c)
	}
	if len(c.FilterSubjects) != 1 || c.FilterSubjects[0] != "dcs.processed" {
		t.Errorf("filter subjects = %v", c.FilterSubjects)
	}
}

func TestListConsumersUnknownStream(t *testing.T) {
	js, ctx := setup(t)
	if _, err := jsx.New(js).ListConsumers(ctx, "NOPE"); err == nil {
		t.Fatal("expected an error for an unknown stream")
	}
}

func TestGetMessages(t *testing.T) {
	js, ctx := setup(t)

	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name: "MSGS", Subjects: []string{"msgs.>"}, Storage: jetstream.MemoryStorage,
	}); err != nil {
		t.Fatalf("create stream: %v", err)
	}

	for i := 1; i <= 5; i++ {
		msg := nats.NewMsg(fmt.Sprintf("msgs.%d", i))
		msg.Data = []byte(fmt.Sprintf(`{"id":%d}`, i))
		msg.Header.Set("X-Custom", fmt.Sprintf("val-%d", i))
		if _, err := js.PublishMsg(ctx, msg); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	svc := jsx.New(js)
	msgs, err := svc.GetMessages(ctx, "MSGS", jsx.GetMessagesOptions{StartSeq: 1, Limit: 3})
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	if msgs[0].Seq != 1 || msgs[0].Subject != "msgs.1" || msgs[0].Data != `{"id":1}` {
		t.Errorf("unexpected msg 0: %+v", msgs[0])
	}
	if msgs[0].Headers["X-Custom"] != "val-1" {
		t.Errorf("missing header in msg 0: %+v", msgs[0].Headers)
	}

	// Test pagination startSeq
	msgsPage2, err := svc.GetMessages(ctx, "MSGS", jsx.GetMessagesOptions{StartSeq: 4, Limit: 10})
	if err != nil {
		t.Fatalf("GetMessages page 2: %v", err)
	}
	if len(msgsPage2) != 2 {
		t.Fatalf("got %d messages for page 2, want 2", len(msgsPage2))
	}
	if msgsPage2[0].Seq != 4 || msgsPage2[1].Seq != 5 {
		t.Errorf("unexpected seqs: %d, %d", msgsPage2[0].Seq, msgsPage2[1].Seq)
	}
}
