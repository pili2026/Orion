package service

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryDLQEnqueueDropsWhenFull(t *testing.T) {
	q := NewInMemoryDLQ(1)
	q.Enqueue(DLQMessage{MQTTUsername: "a"})
	q.Enqueue(DLQMessage{MQTTUsername: "b"}) // should drop, not panic
	if got := len(q.ch); got != 1 {
		t.Fatalf("expected 1 message in channel, got %d", got)
	}
}

func TestInMemoryDLQRunExitsOnCtxCancel(t *testing.T) {
	q := NewInMemoryDLQ(8)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		q.Run(ctx, func(ctx context.Context, msg DLQMessage) {})
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after ctx cancel")
	}
}

func TestInMemoryDLQRunExitsOnClose(t *testing.T) {
	q := NewInMemoryDLQ(8)
	done := make(chan struct{})
	go func() {
		q.Run(context.Background(), func(ctx context.Context, msg DLQMessage) {})
		close(done)
	}()
	q.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after Close")
	}
}
