//go:build jetstream

// Package jetstream is the optional NATS JetStream Broker adapter (#135
// PoC). It is compiled only with `-tags jetstream`; the default build and
// every Core package depend on nothing from NATS.
//
// Tasks go to a work-queue stream (durable, at-least-once, explicit ack,
// bounded redelivery). Results go to core NATS subject
// insight.results.<jobID>: they are not persisted, because a coordinator
// that misses one simply times out and retries the partition with a new
// fencing token, and the work is deterministic.
package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"insight-lab/internal/execution"
)

const (
	streamName    = "INSIGHT_WORK"
	taskSubject   = "insight.work"
	consumerName  = "insight-workers"
	resultSubject = "insight.results."
	// DefaultAckWait is how long a worker may hold a task before it is
	// redelivered; override with ?ackWait=<duration> on the broker URL.
	DefaultAckWait = 5 * time.Minute
	// MaxDeliver bounds redelivery of a task no worker can finish; the
	// coordinator then times out and fails or retries the partition.
	MaxDeliver = 5
)

func init() { execution.RegisterBroker("nats", Open) }

type broker struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	consumer jetstream.Consumer
}

// Open connects to rawURL (nats://host:port) and ensures the stream and
// the shared worker consumer exist. Credentials, if any, come from the URL
// or the NATS environment of the operator, never from a WorkSpec.
func Open(ctx context.Context, rawURL string) (execution.Broker, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid broker URL: %w", err)
	}
	ackWait := DefaultAckWait
	if v := u.Query().Get("ackWait"); v != "" {
		if ackWait, err = time.ParseDuration(v); err != nil || ackWait <= 0 {
			return nil, fmt.Errorf("invalid ackWait %q", v)
		}
	}
	u.RawQuery = ""
	rawURL = u.String()
	nc, err := nats.Connect(rawURL, nats.Name("insight-lab"))
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", redact(rawURL), err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: streamName, Subjects: []string{taskSubject}, Retention: jetstream.WorkQueuePolicy, Storage: jetstream.FileStorage,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure stream: %w", err)
	}
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: consumerName, AckPolicy: jetstream.AckExplicitPolicy, AckWait: ackWait, MaxDeliver: MaxDeliver,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure consumer: %w", err)
	}
	return &broker{nc: nc, js: js, consumer: consumer}, nil
}

func (b *broker) PublishTask(ctx context.Context, t execution.Task) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(ctx, taskSubject, data)
	return err
}

func (b *broker) NextTask(ctx context.Context) (execution.Task, func() error, error) {
	for {
		if err := ctx.Err(); err != nil {
			return execution.Task{}, nil, err
		}
		batch, err := b.consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			return execution.Task{}, nil, err
		}
		for msg := range batch.Messages() {
			var t execution.Task
			if err := json.Unmarshal(msg.Data(), &t); err != nil {
				_ = msg.Term() // undecodable: never deliverable
				continue
			}
			return t, msg.Ack, nil
		}
		if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) {
			return execution.Task{}, nil, err
		}
	}
}

func (b *broker) PublishResult(_ context.Context, r execution.TaskResult) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := b.nc.Publish(resultSubject+subjectToken(r.Result.JobID), data); err != nil {
		return err
	}
	return b.nc.Flush()
}

func (b *broker) SubscribeResults(_ context.Context, jobID string) (<-chan execution.TaskResult, func(), error) {
	raw := make(chan *nats.Msg, 64)
	sub, err := b.nc.ChanSubscribe(resultSubject+subjectToken(jobID), raw)
	if err != nil {
		return nil, nil, err
	}
	if err := b.nc.Flush(); err != nil { // the subscription is live before the task is published
		_ = sub.Unsubscribe()
		return nil, nil, err
	}
	out := make(chan execution.TaskResult, 64)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case m := <-raw:
				var r execution.TaskResult
				if json.Unmarshal(m.Data, &r) == nil {
					select {
					case out <- r:
					default:
					}
				}
			}
		}
	}()
	stop := func() { _ = sub.Unsubscribe(); close(done) }
	return out, stop, nil
}

func (b *broker) Close() error {
	b.nc.Close()
	return nil
}

// subjectToken keeps a job ID a single NATS subject token.
func subjectToken(id string) string {
	return strings.NewReplacer(".", "_", "*", "_", ">", "_", " ", "_").Replace(id)
}

// redact drops user info from a broker URL before it is logged.
func redact(rawURL string) string {
	if i := strings.Index(rawURL, "@"); i >= 0 {
		if j := strings.Index(rawURL, "://"); j >= 0 && j < i {
			return rawURL[:j+3] + "***@" + rawURL[i+1:]
		}
	}
	return rawURL
}
