// Command natsradar-seed creates demo streams and consumers so there is
// something to look at in the UI. It models an "orders" service producing events
// that a "payments" service consumes, and the reverse.
//
// Result after running:
//
//	ORDERS   stream: 25 messages, consumer payments-processor -> 15 pending, 4 ack pending
//	PAYMENTS stream:  5 messages, consumer orders-worker      ->  5 pending
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	url := flag.String("url", "nats://127.0.0.1:4222", "NATS server URL")
	flag.Parse()
	if err := run(*url); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(url string) error {
	nc, err := nats.Connect(url, nats.Name("natsradar-seed"), nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, cfg := range []jetstream.StreamConfig{
		{Name: "ORDERS", Subjects: []string{"orders.>"}, Storage: jetstream.FileStorage, MaxAge: 7 * 24 * time.Hour, Duplicates: 2 * time.Minute},
		{Name: "PAYMENTS", Subjects: []string{"payments.>"}, Storage: jetstream.FileStorage, MaxAge: 7 * 24 * time.Hour, Duplicates: 2 * time.Minute},
	} {
		if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("stream %s: %w", cfg.Name, err)
		}
	}

	paymentsConsumer, err := js.CreateOrUpdateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{
		Durable: "payments-processor", FilterSubject: "orders.>",
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: 5,
	})
	if err != nil {
		return fmt.Errorf("consumer payments-processor: %w", err)
	}
	if _, err := js.CreateOrUpdateConsumer(ctx, "PAYMENTS", jetstream.ConsumerConfig{
		Durable: "orders-worker", FilterSubject: "payments.>",
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: 5,
	}); err != nil {
		return fmt.Errorf("consumer orders-worker: %w", err)
	}

	for i := 1; i <= 25; i++ {
		payload := fmt.Sprintf(`{"orderId":"ORD-%d","amount":%.2f,"status":"CREATED"}`, i, float64(i)*19.99)
		if _, err := js.Publish(ctx, "orders.created", []byte(payload), jetstream.WithMsgID(fmt.Sprintf("seed-order-%d", i))); err != nil {
			return fmt.Errorf("publish orders.created: %w", err)
		}
	}
	for i := 1; i <= 5; i++ {
		payload := fmt.Sprintf(`{"paymentId":"PAY-%d","status":"PROCESSED"}`, i)
		if _, err := js.Publish(ctx, "payments.processed", []byte(payload), jetstream.WithMsgID(fmt.Sprintf("seed-payment-%d", i))); err != nil {
			return fmt.Errorf("publish payments.processed: %w", err)
		}
	}

	// payments-processor fetches 10 messages and acks 6, leaving 4 "in flight" (ack pending).
	batch, err := paymentsConsumer.Fetch(10, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	n := 0
	for msg := range batch.Messages() {
		if n < 6 {
			if err := msg.DoubleAck(ctx); err != nil {
				return fmt.Errorf("ack: %w", err)
			}
		}
		n++
	}
	if err := batch.Error(); err != nil {
		return fmt.Errorf("fetch batch: %w", err)
	}

	fmt.Println("seeded: streams ORDERS, PAYMENTS; consumers payments-processor (on ORDERS), orders-worker (on PAYMENTS)")

	// Seed Key-Value stores
	appConfigs, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      "app-configs",
		Description: "Production microservice configurations",
		History:     5,
	})
	if err != nil {
		return fmt.Errorf("kv app-configs: %w", err)
	}
	_, _ = appConfigs.Put(ctx, "database", []byte(`{"host":"postgres.internal","port":5432,"pool_size":20,"ssl":true}`))
	_, _ = appConfigs.Put(ctx, "redis", []byte(`{"cluster":["redis-0:6379","redis-1:6379"],"tls":false}`))
	_, _ = appConfigs.Put(ctx, "auth.jwt", []byte(`{"expiry_hours":24,"issuer":"auth.company.internal"}`))

	featureFlags, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      "feature-flags",
		Description: "Dynamic canary and release flags",
		History:     10,
	})
	if err != nil {
		return fmt.Errorf("kv feature-flags: %w", err)
	}
	// Write twice to generate revision history
	_, _ = featureFlags.Put(ctx, "payment_v2", []byte(`{"enabled":false,"traffic_pct":0}`))
	_, _ = featureFlags.Put(ctx, "payment_v2", []byte(`{"enabled":true,"traffic_pct":25}`))
	_, _ = featureFlags.Put(ctx, "dark_mode_default", []byte(`{"enabled":true}`))

	fmt.Println("seeded: KV buckets app-configs, feature-flags (with revision history)")

	// Seed Object Store
	reports, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Bucket:      "reports",
		Description: "Generated business and audit reports",
	})
	if err != nil {
		return fmt.Errorf("object store reports: %w", err)
	}
	_, _ = reports.PutBytes(ctx, "monthly-summary.json", []byte(`{
  "month": "October",
  "transactions": 14250,
  "volume_usd": 2489100.50,
  "currency": "USD"
}`))
	_, _ = reports.PutBytes(ctx, "audit-log.csv", []byte("timestamp,action,user,ip\n2026-11-10T10:00:00Z,LOGIN,admin,10.0.1.5\n2026-11-10T10:05:00Z,STREAM_CREATE,admin,10.0.1.5\n"))

	fmt.Println("seeded: Object Store bucket reports (monthly-summary.json, audit-log.csv)")
	return nil
}
