// Package deleter implements an asynchronous, batching pipeline that marks
// shortened URLs as deleted. Many HTTP handlers can submit jobs concurrently;
// per-job producer goroutines fan their items into a single internal channel
// that one consumer drains, buffers, and flushes via a batch update.
package deleter

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Storer is the subset of storage operations the deleter relies on.
type Storer interface {
	DeleteUserURLs(codes []string, userID string) error
}

type item struct {
	code   string
	userID string
}

// Deleter accepts deletion jobs and flushes them in batches.
type Deleter struct {
	storage       Storer
	log           *zap.Logger
	in            chan item
	flushSize     int
	flushInterval time.Duration

	wg     sync.WaitGroup
	stop   chan struct{}
	closed sync.Once
}

// New constructs a Deleter. flushSize is the max number of buffered items
// before a forced flush; flushInterval is the max delay before a non-empty
// buffer is flushed even when not full.
func New(storage Storer, log *zap.Logger, flushSize int, flushInterval time.Duration) *Deleter {
	if flushSize <= 0 {
		flushSize = 64
	}
	if flushInterval <= 0 {
		flushInterval = time.Second
	}
	return &Deleter{
		storage:       storage,
		log:           log,
		in:            make(chan item, flushSize*2),
		flushSize:     flushSize,
		flushInterval: flushInterval,
		stop:          make(chan struct{}),
	}
}

// Start launches the consumer goroutine. Call Stop to drain and shut down.
func (d *Deleter) Start() {
	d.wg.Add(1)
	go d.run()
}

// Enqueue schedules codes belonging to userID for asynchronous deletion.
// Returns immediately; sending is performed by a per-call goroutine so the
// caller is never blocked by a slow consumer.
func (d *Deleter) Enqueue(codes []string, userID string) {
	if len(codes) == 0 || userID == "" {
		return
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for _, code := range codes {
			select {
			case d.in <- item{code: code, userID: userID}:
			case <-d.stop:
				return
			}
		}
	}()
}

// Stop signals shutdown, drains buffered work with the provided context, and
// returns once all goroutines have exited or ctx is cancelled.
func (d *Deleter) Stop(ctx context.Context) {
	d.closed.Do(func() { close(d.stop) })
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (d *Deleter) run() {
	defer d.wg.Done()

	buf := make([]item, 0, d.flushSize)
	ticker := time.NewTicker(d.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(buf) == 0 {
			return
		}
		byUser := make(map[string][]string, 1)
		for _, it := range buf {
			byUser[it.userID] = append(byUser[it.userID], it.code)
		}
		for uid, codes := range byUser {
			if err := d.storage.DeleteUserURLs(codes, uid); err != nil {
				d.log.Error("failed to mark urls deleted",
					zap.String("user_id", uid),
					zap.Int("count", len(codes)),
					zap.Error(err),
				)
			}
		}
		buf = buf[:0]
	}

	for {
		select {
		case it := <-d.in:
			buf = append(buf, it)
			if len(buf) >= d.flushSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-d.stop:
			// Drain anything still buffered in the channel before exiting.
			for {
				select {
				case it := <-d.in:
					buf = append(buf, it)
					if len(buf) >= d.flushSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}
