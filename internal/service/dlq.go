package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/hill/orion/internal/dto"
)

const DLQBufferSize = 1024

// DLQMessage carries a single failed TelemetryReading for retry.
type DLQMessage struct {
	MQTTUsername string
	TS           time.Time
	ReceivedAt   time.Time
	Reading      dto.TelemetryReading
	Retries      int
}

// DLQ is the interface for a dead-letter queue.
type DLQ interface {
	Enqueue(msg DLQMessage)
	Run(ctx context.Context, fn func(ctx context.Context, msg DLQMessage))
	Close()
}

// InMemoryDLQ is a channel-backed DLQ implementation.
type InMemoryDLQ struct {
	ch chan DLQMessage
}

// NewInMemoryDLQ creates a new InMemoryDLQ with the given buffer size.
func NewInMemoryDLQ(bufSize int) *InMemoryDLQ {
	return &InMemoryDLQ{ch: make(chan DLQMessage, bufSize)}
}

// Enqueue adds a message to the queue. If the queue is full, the message is dropped.
func (q *InMemoryDLQ) Enqueue(msg DLQMessage) {
	select {
	case q.ch <- msg:
	default:
		slog.Error("mqtt_ingest: DLQ channel full, dropping reading",
			slog.String("mqtt_username", msg.MQTTUsername),
			slog.String("type", msg.Reading.Type),
			slog.String("device_code", msg.Reading.DeviceCode),
			slog.Int("retries", msg.Retries),
		)
	}
}

// Run processes messages from the queue using fn until ctx is cancelled or the channel is closed.
func (q *InMemoryDLQ) Run(ctx context.Context, fn func(ctx context.Context, msg DLQMessage)) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-q.ch:
			if !ok {
				return
			}
			fn(ctx, msg)
		}
	}
}

// Close closes the underlying channel.
func (q *InMemoryDLQ) Close() {
	close(q.ch)
}
