// Package jsx is the JetStream service layer: read-only views over streams and
// consumers, shaped for the NATSRadar API.
package jsx

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go/jetstream"
)

// maxParallel bounds concurrent consumer listings so a cluster with hundreds of
// streams is not flooded with API requests.
const maxParallel = 8

// Service reads JetStream state. It never changes server state.
type Service struct {
	js jetstream.JetStream
}

// New creates a Service over the given JetStream handle.
func New(js jetstream.JetStream) *Service {
	return &Service{js: js}
}

// StreamSummary is one row of the streams list, including consumer lag rolled
// up across all consumers of the stream.
type StreamSummary struct {
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Subjects      []string  `json:"subjects"`
	Storage       string    `json:"storage"`
	Retention     string    `json:"retention"`
	Replicas      int       `json:"replicas"`
	Messages      uint64    `json:"messages"`
	Bytes         uint64    `json:"bytes"`
	FirstSeq      uint64    `json:"firstSeq"`
	LastSeq       uint64    `json:"lastSeq"`
	ConsumerCount int       `json:"consumerCount"`
	Pending       uint64    `json:"pending"`
	AckPending    int       `json:"ackPending"`
	Redelivered   int       `json:"redelivered"`
	LagError      string    `json:"lagError,omitempty"`
	Created       time.Time `json:"created"`
}

// ConsumerSummary describes one consumer and its live counters.
type ConsumerSummary struct {
	Name               string     `json:"name"`
	Durable            bool       `json:"durable"`
	Description        string     `json:"description,omitempty"`
	FilterSubjects     []string   `json:"filterSubjects"`
	DeliverPolicy      string     `json:"deliverPolicy"`
	AckPolicy          string     `json:"ackPolicy"`
	AckWaitMs          int64      `json:"ackWaitMs"`
	MaxDeliver         int        `json:"maxDeliver"`
	Pending            uint64     `json:"pending"`
	AckPending         int        `json:"ackPending"`
	Redelivered        int        `json:"redelivered"`
	Waiting            int        `json:"waiting"`
	DeliveredStreamSeq uint64     `json:"deliveredStreamSeq"`
	AckFloorStreamSeq  uint64     `json:"ackFloorStreamSeq"`
	LastDelivered      *time.Time `json:"lastDelivered,omitempty"`
	Created            time.Time  `json:"created"`
}

// ListStreams returns every stream with its consumer lag rolled up.
func (s *Service) ListStreams(ctx context.Context) ([]StreamSummary, error) {
	lister := s.js.ListStreams(ctx)
	out := make([]StreamSummary, 0)
	for info := range lister.Info() {
		out = append(out, summarizeStream(info))
	}
	if err := lister.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	// Roll up consumer counters per stream, a few streams at a time.
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i := range out {
		if out[i].ConsumerCount == 0 {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(row *StreamSummary) {
			defer wg.Done()
			defer func() { <-sem }()
			consumers, err := s.ListConsumers(ctx, row.Name)
			if err != nil {
				row.LagError = err.Error()
				return
			}
			for _, c := range consumers {
				row.Pending += c.Pending
				row.AckPending += c.AckPending
				row.Redelivered += c.Redelivered
			}
		}(&out[i])
	}
	wg.Wait()
	return out, nil
}

// ListConsumers returns the consumers of one stream.
func (s *Service) ListConsumers(ctx context.Context, stream string) ([]ConsumerSummary, error) {
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("stream %q: %w", stream, err)
	}
	lister := st.ListConsumers(ctx)
	out := make([]ConsumerSummary, 0)
	for info := range lister.Info() {
		out = append(out, summarizeConsumer(info))
	}
	if err := lister.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// PurgeStream removes all messages from a stream without deleting the stream configuration.
func (s *Service) PurgeStream(ctx context.Context, stream string) error {
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return fmt.Errorf("stream %q: %w", stream, err)
	}
	return st.Purge(ctx)
}

// DeleteStream permanently deletes the stream.
func (s *Service) DeleteStream(ctx context.Context, stream string) error {
	return s.js.DeleteStream(ctx, stream)
}

// CreateStreamConfig describes parameters for creating a new stream.
type CreateStreamConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Subjects    []string `json:"subjects"`
	Storage     string   `json:"storage,omitempty"`
	Retention   string   `json:"retention,omitempty"`
	Replicas    int      `json:"replicas,omitempty"`
}

// CreateStream creates a new JetStream stream.
func (s *Service) CreateStream(ctx context.Context, cfg CreateStreamConfig) (*StreamSummary, error) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return nil, errors.New("stream name is required")
	}
	if len(cfg.Subjects) == 0 {
		cfg.Subjects = []string{cfg.Name + ".>"}
	}
	storage := jetstream.FileStorage
	if cfg.Storage == "memory" {
		storage = jetstream.MemoryStorage
	}
	retention := jetstream.LimitsPolicy
	switch cfg.Retention {
	case "interest":
		retention = jetstream.InterestPolicy
	case "workqueue":
		retention = jetstream.WorkQueuePolicy
	}
	replicas := cfg.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	st, err := s.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Subjects:    cfg.Subjects,
		Storage:     storage,
		Retention:   retention,
		Replicas:    replicas,
	})
	if err != nil {
		return nil, err
	}
	info, err := st.Info(ctx)
	if err != nil {
		return nil, err
	}
	summary := summarizeStream(info)
	return &summary, nil
}

// maxMessages caps the number of messages returned in a single browse request.
const maxMessages = 50

// MessageSummary is one message fetched by direct-get. Browsing never creates
// consumers, never acks, and never changes server state.
type MessageSummary struct {
	Seq       uint64            `json:"seq"`
	Subject   string            `json:"subject"`
	Data      string            `json:"data"`
	IsBinary  bool              `json:"isBinary"`
	Headers   map[string]string `json:"headers,omitempty"`
	Size      int               `json:"size"`
	Timestamp time.Time         `json:"timestamp"`
}

// GetMessagesOptions controls message fetching, subject filtering, text search and time-travel.
type GetMessagesOptions struct {
	StartSeq      uint64
	StartTime     *time.Time
	FilterSubject string
	Search        string
	Limit         int
}

// GetMessages fetches messages from the given stream by sequence range using
// the stream's direct-get API. If StartTime is specified, it binary-searches
// for the nearest starting sequence.
// This is a pure read — no consumer is created and no server state changes.
func (s *Service) GetMessages(ctx context.Context, stream string, opt GetMessagesOptions) ([]MessageSummary, error) {
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("stream %q: %w", stream, err)
	}
	info, err := st.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream info %q: %w", stream, err)
	}

	if info.State.Msgs == 0 {
		return []MessageSummary{}, nil
	}

	startSeq := opt.StartSeq
	if opt.StartTime != nil && !opt.StartTime.IsZero() {
		startSeq = s.findSeqByTime(ctx, st, *opt.StartTime, info.State.FirstSeq, info.State.LastSeq)
	} else if startSeq == 0 || startSeq < info.State.FirstSeq {
		startSeq = info.State.FirstSeq
	}

	limit := opt.Limit
	if limit <= 0 || limit > maxMessages {
		limit = maxMessages
	}

	filterSubject := strings.TrimSpace(opt.FilterSubject)
	searchTerm := strings.ToLower(strings.TrimSpace(opt.Search))

	out := make([]MessageSummary, 0, limit)
	// Scan forward up to 1000 messages or until limit is reached
	scanCap := startSeq + 2000
	if scanCap > info.State.LastSeq+1 {
		scanCap = info.State.LastSeq + 1
	}

	for seq := startSeq; seq < scanCap && len(out) < limit; seq++ {
		msg, err := st.GetMsg(ctx, seq)
		if err != nil {
			continue
		}
		if filterSubject != "" && !matchSubject(filterSubject, msg.Subject) {
			continue
		}
		if searchTerm != "" {
			subjectMatch := strings.Contains(strings.ToLower(msg.Subject), searchTerm)
			dataMatch := strings.Contains(strings.ToLower(string(msg.Data)), searchTerm)
			headerMatch := false
			for k, v := range msg.Header {
				if strings.Contains(strings.ToLower(k), searchTerm) {
					headerMatch = true
					break
				}
				for _, val := range v {
					if strings.Contains(strings.ToLower(val), searchTerm) {
						headerMatch = true
						break
					}
				}
			}
			if !subjectMatch && !dataMatch && !headerMatch {
				continue
			}
		}
		out = append(out, summarizeMessage(seq, msg))
	}
	return out, nil
}

func (s *Service) findSeqByTime(ctx context.Context, st jetstream.Stream, target time.Time, first, last uint64) uint64 {
	low := first
	high := last
	ans := first
	for low <= high {
		mid := low + (high-low)/2
		msg, err := st.GetMsg(ctx, mid)
		if err != nil {
			low = mid + 1
			continue
		}
		if msg.Time.Before(target) {
			low = mid + 1
		} else {
			ans = mid
			if mid == 0 {
				break
			}
			high = mid - 1
		}
	}
	return ans
}

func matchSubject(pattern, subject string) bool {
	if pattern == "" || pattern == ">" || pattern == subject {
		return true
	}
	pTokens := strings.Split(pattern, ".")
	sTokens := strings.Split(subject, ".")
	for i, p := range pTokens {
		if p == ">" {
			return true
		}
		if i >= len(sTokens) {
			return false
		}
		if p != "*" && p != sTokens[i] {
			return false
		}
	}
	return len(pTokens) == len(sTokens)
}

func summarizeMessage(seq uint64, msg *jetstream.RawStreamMsg) MessageSummary {
	data := string(msg.Data)
	isBinary := !utf8.Valid(msg.Data)
	if isBinary {
		data = base64.StdEncoding.EncodeToString(msg.Data)
	}

	headers := make(map[string]string)
	for k, v := range msg.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var h map[string]string
	if len(headers) > 0 {
		h = headers
	}

	return MessageSummary{
		Seq:       seq,
		Subject:   msg.Subject,
		Data:      data,
		IsBinary:  isBinary,
		Headers:   h,
		Size:      len(msg.Data),
		Timestamp: msg.Time,
	}
}

// The client's String() methods are not stable across types (for example
// AckExplicitPolicy prints "AckExplicit"), so map the values explicitly to the
// lowercase names NATS itself uses.

func ackPolicyName(p jetstream.AckPolicy) string {
	switch p {
	case jetstream.AckNonePolicy:
		return "none"
	case jetstream.AckAllPolicy:
		return "all"
	case jetstream.AckExplicitPolicy:
		return "explicit"
	default:
		return strings.ToLower(p.String())
	}
}

func deliverPolicyName(p jetstream.DeliverPolicy) string {
	switch p {
	case jetstream.DeliverAllPolicy:
		return "all"
	case jetstream.DeliverLastPolicy:
		return "last"
	case jetstream.DeliverNewPolicy:
		return "new"
	case jetstream.DeliverByStartSequencePolicy:
		return "by_start_sequence"
	case jetstream.DeliverByStartTimePolicy:
		return "by_start_time"
	case jetstream.DeliverLastPerSubjectPolicy:
		return "last_per_subject"
	default:
		return strings.ToLower(p.String())
	}
}

func retentionName(p jetstream.RetentionPolicy) string {
	switch p {
	case jetstream.LimitsPolicy:
		return "limits"
	case jetstream.InterestPolicy:
		return "interest"
	case jetstream.WorkQueuePolicy:
		return "workqueue"
	default:
		return strings.ToLower(p.String())
	}
}

func storageName(t jetstream.StorageType) string {
	switch t {
	case jetstream.FileStorage:
		return "file"
	case jetstream.MemoryStorage:
		return "memory"
	default:
		return strings.ToLower(t.String())
	}
}

func summarizeStream(info *jetstream.StreamInfo) StreamSummary {
	subjects := info.Config.Subjects
	if subjects == nil {
		subjects = []string{}
	}
	return StreamSummary{
		Name:          info.Config.Name,
		Description:   info.Config.Description,
		Subjects:      subjects,
		Storage:       storageName(info.Config.Storage),
		Retention:     retentionName(info.Config.Retention),
		Replicas:      info.Config.Replicas,
		Messages:      info.State.Msgs,
		Bytes:         info.State.Bytes,
		FirstSeq:      info.State.FirstSeq,
		LastSeq:       info.State.LastSeq,
		ConsumerCount: info.State.Consumers,
		Created:       info.Created,
	}
}

func summarizeConsumer(info *jetstream.ConsumerInfo) ConsumerSummary {
	cfg := info.Config
	filters := make([]string, 0, len(cfg.FilterSubjects)+1)
	if cfg.FilterSubject != "" {
		filters = append(filters, cfg.FilterSubject)
	}
	filters = append(filters, cfg.FilterSubjects...)

	return ConsumerSummary{
		Name:               info.Name,
		Durable:            cfg.Durable != "",
		Description:        cfg.Description,
		FilterSubjects:     filters,
		DeliverPolicy:      deliverPolicyName(cfg.DeliverPolicy),
		AckPolicy:          ackPolicyName(cfg.AckPolicy),
		AckWaitMs:          cfg.AckWait.Milliseconds(),
		MaxDeliver:         cfg.MaxDeliver,
		Pending:            info.NumPending,
		AckPending:         info.NumAckPending,
		Redelivered:        info.NumRedelivered,
		Waiting:            info.NumWaiting,
		DeliveredStreamSeq: info.Delivered.Stream,
		AckFloorStreamSeq:  info.AckFloor.Stream,
		LastDelivered:      info.Delivered.Last,
		Created:            info.Created,
	}
}

// ---- Consumer Management ----

func (s *Service) CreateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (*ConsumerSummary, error) {
	c, err := s.js.CreateOrUpdateConsumer(ctx, stream, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}
	info, err := c.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("consumer info: %w", err)
	}
	sum := summarizeConsumer(info)
	return &sum, nil
}

func (s *Service) DeleteConsumer(ctx context.Context, stream, consumer string) error {
	return s.js.DeleteConsumer(ctx, stream, consumer)
}

func (s *Service) PauseConsumer(ctx context.Context, stream, consumer string, pauseUntil time.Time) error {
	_, err := s.js.PauseConsumer(ctx, stream, consumer, pauseUntil)
	return err
}

func (s *Service) ResumeConsumer(ctx context.Context, stream, consumer string) error {
	_, err := s.js.ResumeConsumer(ctx, stream, consumer)
	return err
}

// ---- KeyValue Store ----

type KVBucketSummary struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description,omitempty"`
	Values      uint64 `json:"values"`
	History     int64  `json:"history"`
	TTLMs       int64  `json:"ttlMs"`
	Bytes       uint64 `json:"bytes"`
	Replicas    int    `json:"replicas"`
}

type KVKeyEntry struct {
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	IsBinary  bool      `json:"isBinary"`
	Revision  uint64    `json:"revision"`
	Created   time.Time `json:"created"`
	Operation string    `json:"operation"`
}

func (s *Service) ListKVBuckets(ctx context.Context) ([]KVBucketSummary, error) {
	lister := s.js.KeyValueStores(ctx)
	var out []KVBucketSummary
	for status := range lister.Status() {
		if status == nil {
			continue
		}
		out = append(out, KVBucketSummary{
			Bucket:      status.Bucket(),
			Description: status.Config().Description,
			Values:      status.Values(),
			History:     status.History(),
			TTLMs:       status.TTL().Milliseconds(),
			Bytes:       status.Bytes(),
			Replicas:    status.Config().Replicas,
		})
	}
	if err := lister.Error(); err != nil && !errors.Is(err, jetstream.ErrBucketNotFound) {
		return nil, err
	}
	if out == nil {
		out = []KVBucketSummary{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	return out, nil
}

func (s *Service) CreateKVBucket(ctx context.Context, cfg jetstream.KeyValueConfig) error {
	_, err := s.js.CreateKeyValue(ctx, cfg)
	return err
}

func (s *Service) DeleteKVBucket(ctx context.Context, bucket string) error {
	return s.js.DeleteKeyValue(ctx, bucket)
}

func (s *Service) ListKVKeys(ctx context.Context, bucket string) ([]string, error) {
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, err
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return []string{}, nil
		}
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *Service) GetKVKey(ctx context.Context, bucket, key string) (*KVKeyEntry, error) {
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, err
	}
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return summarizeKVEntry(entry), nil
}

func (s *Service) PutKVKey(ctx context.Context, bucket, key string, val []byte) (uint64, error) {
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return 0, err
	}
	return kv.Put(ctx, key, val)
}

func (s *Service) DeleteKVKey(ctx context.Context, bucket, key string) error {
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return err
	}
	return kv.Delete(ctx, key)
}

func (s *Service) GetKVHistory(ctx context.Context, bucket, key string) ([]KVKeyEntry, error) {
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, err
	}
	history, err := kv.History(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]KVKeyEntry, 0, len(history))
	for _, entry := range history {
		out = append(out, *summarizeKVEntry(entry))
	}
	return out, nil
}

func summarizeKVEntry(entry jetstream.KeyValueEntry) *KVKeyEntry {
	val := entry.Value()
	isBinary := !utf8.Valid(val)
	strVal := string(val)
	if isBinary {
		strVal = base64.StdEncoding.EncodeToString(val)
	}
	op := "PUT"
	switch entry.Operation() {
	case jetstream.KeyValueDelete:
		op = "DEL"
	case jetstream.KeyValuePurge:
		op = "PURGE"
	}
	return &KVKeyEntry{
		Bucket:    entry.Bucket(),
		Key:       entry.Key(),
		Value:     strVal,
		IsBinary:  isBinary,
		Revision:  entry.Revision(),
		Created:   entry.Created(),
		Operation: op,
	}
}

// ---- Object Store ----

type ObjectBucketSummary struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description,omitempty"`
	Size        uint64 `json:"size"`
	Storage     string `json:"storage"`
	Replicas    int    `json:"replicas"`
	Sealed      bool   `json:"sealed"`
}

type ObjectSummary struct {
	Name    string    `json:"name"`
	Size    uint64    `json:"size"`
	ModTime time.Time `json:"modTime"`
	Digest  string    `json:"digest,omitempty"`
	Deleted bool      `json:"deleted"`
}

func (s *Service) ListObjectBuckets(ctx context.Context) ([]ObjectBucketSummary, error) {
	lister := s.js.ObjectStores(ctx)
	var out []ObjectBucketSummary
	for status := range lister.Status() {
		if status == nil {
			continue
		}
		out = append(out, ObjectBucketSummary{
			Bucket:      status.Bucket(),
			Description: status.Description(),
			Size:        status.Size(),
			Storage:     storageName(status.Storage()),
			Replicas:    status.Replicas(),
			Sealed:      status.Sealed(),
		})
	}
	if err := lister.Error(); err != nil && !errors.Is(err, jetstream.ErrBucketNotFound) {
		return nil, err
	}
	if out == nil {
		out = []ObjectBucketSummary{}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	return out, nil
}

func (s *Service) CreateObjectBucket(ctx context.Context, cfg jetstream.ObjectStoreConfig) error {
	_, err := s.js.CreateObjectStore(ctx, cfg)
	return err
}

func (s *Service) DeleteObjectBucket(ctx context.Context, bucket string) error {
	return s.js.DeleteObjectStore(ctx, bucket)
}

func (s *Service) ListObjects(ctx context.Context, bucket string) ([]ObjectSummary, error) {
	obs, err := s.js.ObjectStore(ctx, bucket)
	if err != nil {
		return nil, err
	}
	list, err := obs.List(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoObjectsFound) {
			return []ObjectSummary{}, nil
		}
		return nil, err
	}
	out := make([]ObjectSummary, 0, len(list))
	for _, obj := range list {
		out = append(out, ObjectSummary{
			Name:    obj.Name,
			Size:    obj.Size,
			ModTime: obj.ModTime,
			Digest:  obj.Digest,
			Deleted: obj.Deleted,
		})
	}
	return out, nil
}

func (s *Service) GetObject(ctx context.Context, bucket, name string) ([]byte, *jetstream.ObjectInfo, error) {
	obs, err := s.js.ObjectStore(ctx, bucket)
	if err != nil {
		return nil, nil, err
	}
	info, err := obs.GetInfo(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	data, err := obs.GetBytes(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

func (s *Service) PutObject(ctx context.Context, bucket, name string, data []byte) (*jetstream.ObjectInfo, error) {
	obs, err := s.js.ObjectStore(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return obs.PutBytes(ctx, name, data)
}

func (s *Service) DeleteObject(ctx context.Context, bucket, name string) error {
	obs, err := s.js.ObjectStore(ctx, bucket)
	if err != nil {
		return err
	}
	return obs.Delete(ctx, name)
}