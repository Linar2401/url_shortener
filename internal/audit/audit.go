// Package audit implements the Observer pattern for audit logging:
// a Publisher (Subject) fans out Events to registered Observers — currently
// a file appender and an HTTP POSTer. Observers run synchronously inside
// Publish; each one isolates its own errors so a misbehaving sink cannot
// silence the others.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Action enumerates the audited request types.
type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

// Event is the payload pushed to every Observer.
type Event struct {
	Ts     int64  `json:"ts"`
	Action Action `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}

// Observer receives audit events.
type Observer interface {
	OnEvent(e Event)
}

// Publisher is the Subject in the Observer pattern: it keeps a list of
// observers and broadcasts events to all of them.
type Publisher struct {
	mu        sync.RWMutex
	observers []Observer
	log       *zap.Logger
}

// NewPublisher returns an empty Publisher. log is used to record observer
// failures so a broken sink is visible without crashing the request path.
func NewPublisher(log *zap.Logger) *Publisher {
	return &Publisher{log: log}
}

// Subscribe registers an observer. Safe to call before Publish but not
// expected to be called concurrently with traffic in this service.
func (p *Publisher) Subscribe(o Observer) {
	if o == nil {
		return
	}
	p.mu.Lock()
	p.observers = append(p.observers, o)
	p.mu.Unlock()
}

// Publish broadcasts e to every registered observer. If no observers are
// configured the call is a no-op. Ts is filled in here if the caller left
// it zero so handlers don't all have to remember to set it.
func (p *Publisher) Publish(e Event) {
	if p == nil {
		return
	}
	if e.Ts == 0 {
		e.Ts = time.Now().Unix()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, o := range p.observers {
		o.OnEvent(e)
	}
}

// FileObserver appends one JSON-encoded event per line to a file.
type FileObserver struct {
	mu  sync.Mutex
	f   *os.File
	log *zap.Logger
}

// NewFileObserver opens path for appending and returns an observer that
// writes events as JSON lines.
func NewFileObserver(path string, log *zap.Logger) (*FileObserver, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("audit: open file: %w", err)
	}
	return &FileObserver{f: f, log: log}, nil
}

func (o *FileObserver) OnEvent(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		o.log.Error("audit: marshal event", zap.Error(err))
		return
	}
	data = append(data, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.f.Write(data); err != nil {
		o.log.Error("audit: write file", zap.Error(err))
	}
}

// Close flushes and closes the underlying file.
func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.f == nil {
		return nil
	}
	err := o.f.Close()
	o.f = nil
	return err
}

// HTTPObserver POSTs each event as JSON to a remote URL.
type HTTPObserver struct {
	url    string
	client *http.Client
	log    *zap.Logger
}

// NewHTTPObserver builds an observer that sends events to url.
func NewHTTPObserver(url string, log *zap.Logger) *HTTPObserver {
	return &HTTPObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
		log:    log,
	}
}

func (o *HTTPObserver) OnEvent(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		o.log.Error("audit: marshal event", zap.Error(err))
		return
	}
	req, err := http.NewRequest(http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		o.log.Error("audit: build request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		o.log.Error("audit: post event", zap.Error(err))
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		o.log.Error("audit: remote rejected event",
			zap.String("url", o.url),
			zap.Int("status", resp.StatusCode),
		)
	}
}
