package deleter

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeStorer struct {
	mu    sync.Mutex
	calls map[string][]string
}

func newFakeStorer() *fakeStorer {
	return &fakeStorer{calls: make(map[string][]string)}
}

func (f *fakeStorer) DeleteUserURLs(codes []string, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[userID] = append(f.calls[userID], codes...)
	return nil
}

func (f *fakeStorer) collected(userID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls[userID]))
	copy(out, f.calls[userID])
	return out
}

func TestDeleter_EnqueueAndFlush(t *testing.T) {
	f := newFakeStorer()
	d := New(f, zap.NewNop(), 100, 10*time.Millisecond)
	d.Start()

	d.Enqueue([]string{"a", "b", "c"}, "alice")
	d.Enqueue(nil, "alice")      // no-op
	d.Enqueue([]string{"x"}, "") // no-op

	// Give producer goroutines time to push into the channel before Stop
	// closes the stop signal (otherwise the producer's select may pick the
	// stop branch and drop pending items).
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d.Stop(ctx)

	got := f.collected("alice")
	if len(got) != 3 {
		t.Fatalf("want 3 codes, got %v", got)
	}
}

func TestDeleter_FlushOnSizeThreshold(t *testing.T) {
	f := newFakeStorer()
	d := New(f, zap.NewNop(), 2, time.Hour) // ticker won't fire
	d.Start()
	d.Enqueue([]string{"a", "b", "c", "d"}, "u")

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	d.Stop(ctx)

	if got := f.collected("u"); len(got) != 4 {
		t.Errorf("want 4 codes flushed, got %v", got)
	}
}

func TestDeleter_DefaultsAppliedForNonPositive(t *testing.T) {
	f := newFakeStorer()
	d := New(f, zap.NewNop(), 0, 0)
	if d.flushSize <= 0 || d.flushInterval <= 0 {
		t.Error("expected New to apply defaults for non-positive args")
	}
}
