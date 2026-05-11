package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

type captureObserver struct {
	mu     sync.Mutex
	events []Event
}

func (c *captureObserver) OnEvent(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func TestPublisher_DispatchesToAllObservers(t *testing.T) {
	p := NewPublisher(zap.NewNop())
	a := &captureObserver{}
	b := &captureObserver{}
	p.Subscribe(a)
	p.Subscribe(b)
	p.Subscribe(nil) // ignored

	p.Publish(Event{Action: ActionShorten, UserID: "u1", URL: "https://x"})

	if len(a.events) != 1 || len(b.events) != 1 {
		t.Fatalf("each observer should receive 1 event, got a=%d b=%d", len(a.events), len(b.events))
	}
	if a.events[0].TS == 0 {
		t.Error("Publisher should auto-fill TS")
	}
	if a.events[0].Action != ActionShorten {
		t.Errorf("wrong action: %s", a.events[0].Action)
	}
}

func TestPublisher_NilSafe(t *testing.T) {
	var p *Publisher
	p.Publish(Event{Action: ActionFollow})
}

func TestFileObserver_AppendsJSONLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path, zap.NewNop())
	if err != nil {
		t.Fatalf("NewFileObserver: %v", err)
	}
	defer func() { _ = obs.Close() }()

	obs.OnEvent(Event{TS: 100, Action: ActionShorten, UserID: "u", URL: "https://e"})
	obs.OnEvent(Event{TS: 101, Action: ActionFollow, UserID: "u", URL: "https://e"})

	if err := obs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), data)
	}
	var first Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if first.Action != ActionShorten || first.URL != "https://e" {
		t.Errorf("unexpected first event: %+v", first)
	}
}

func TestFileObserver_OpenError(t *testing.T) {
	// A path under a non-existent directory cannot be opened for append.
	bad := filepath.Join(t.TempDir(), "nope", "audit.log")
	if _, err := NewFileObserver(bad, zap.NewNop()); err == nil {
		t.Error("expected error opening into non-existent directory")
	}
}

func TestHTTPObserver_PostsJSON(t *testing.T) {
	received := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var e Event
		_ = json.Unmarshal(body, &e)
		received <- e
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL, zap.NewNop())
	obs.OnEvent(Event{TS: 5, Action: ActionShorten, UserID: "u", URL: "https://h"})

	got := <-received
	if got.Action != ActionShorten || got.URL != "https://h" {
		t.Errorf("unexpected event: %+v", got)
	}
}
