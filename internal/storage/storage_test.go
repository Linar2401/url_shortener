package storage

import (
	"errors"
	"path/filepath"
	"sort"
	"testing"
)

func TestSaveAndGetURL(t *testing.T) {
	s, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := s.SaveURL("abc123", "https://example.com", "user-1"); err != nil {
		t.Fatalf("SaveURL: %v", err)
	}

	got, err := s.GetURL("abc123")
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("want https://example.com, got %s", got)
	}

	if _, err := s.GetURL("missing"); err == nil {
		t.Error("expected error for missing code")
	}
}

func TestSaveURL_Conflict(t *testing.T) {
	s, _ := New("")
	if err := s.SaveURL("code1", "https://example.com", "user-1"); err != nil {
		t.Fatalf("first SaveURL: %v", err)
	}

	err := s.SaveURL("code2", "https://example.com", "user-1")
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %v", err)
	}
	if conflict.ShortCode != "code1" {
		t.Errorf("want existing code1, got %s", conflict.ShortCode)
	}
	if conflict.Error() == "" {
		t.Error("ConflictError.Error() returned empty string")
	}
}

func TestSaveURL_Collision(t *testing.T) {
	s, _ := New("")
	if err := s.SaveURL("dup", "https://a.example", "user-1"); err != nil {
		t.Fatalf("first SaveURL: %v", err)
	}
	err := s.SaveURL("dup", "https://b.example", "user-1")
	if !errors.Is(err, ErrCollision) {
		t.Errorf("expected ErrCollision, got %v", err)
	}
}

func TestSaveBatchAndGet(t *testing.T) {
	s, _ := New("")
	items := []BatchItem{
		{ShortCode: "a1", OriginalURL: "https://a.example"},
		{ShortCode: "b1", OriginalURL: "https://b.example"},
	}
	if err := s.SaveBatch(items, "user-1"); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	v, _ := s.GetURL("a1")
	if v != "https://a.example" {
		t.Errorf("a1: got %s", v)
	}
	v, _ = s.GetURL("b1")
	if v != "https://b.example" {
		t.Errorf("b1: got %s", v)
	}

	if err := s.SaveBatch(nil, "user-1"); err != nil {
		t.Errorf("empty batch should be no-op, got %v", err)
	}
}

func TestSaveBatch_DedupesByOriginal(t *testing.T) {
	s, _ := New("")
	_ = s.SaveURL("preexist", "https://x.example", "user-1")

	items := []BatchItem{{ShortCode: "newcode", OriginalURL: "https://x.example"}}
	if err := s.SaveBatch(items, "user-1"); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if items[0].ShortCode != "preexist" {
		t.Errorf("expected dedup to preexist, got %s", items[0].ShortCode)
	}
}

func TestGetUserURLsAndDelete(t *testing.T) {
	s, _ := New("")
	_ = s.SaveURL("u1", "https://1.example", "alice")
	_ = s.SaveURL("u2", "https://2.example", "alice")
	_ = s.SaveURL("u3", "https://3.example", "bob")

	got, err := s.GetUserURLs("alice")
	if err != nil {
		t.Fatalf("GetUserURLs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 urls for alice, got %d", len(got))
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ShortCode < got[j].ShortCode })
	if got[0].ShortCode != "u1" || got[1].ShortCode != "u2" {
		t.Errorf("unexpected codes: %+v", got)
	}

	if err := s.DeleteUserURLs([]string{"u1"}, "alice"); err != nil {
		t.Fatalf("DeleteUserURLs: %v", err)
	}
	if _, err := s.GetURL("u1"); !errors.Is(err, ErrDeleted) {
		t.Errorf("expected ErrDeleted, got %v", err)
	}

	// Deleting another user's code is a no-op.
	if err := s.DeleteUserURLs([]string{"u3"}, "alice"); err != nil {
		t.Fatalf("DeleteUserURLs: %v", err)
	}
	if _, err := s.GetURL("u3"); err != nil {
		t.Errorf("bob's u3 should still be reachable: %v", err)
	}

	// Empty inputs are no-op.
	if err := s.DeleteUserURLs(nil, "alice"); err != nil {
		t.Errorf("empty codes should be no-op: %v", err)
	}
	if err := s.DeleteUserURLs([]string{"u2"}, ""); err != nil {
		t.Errorf("empty userID should be no-op: %v", err)
	}
}

func TestFilePersistence_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "storage.json")

	s1, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s1.SaveURL("xyz", "https://persist.example", "user-1"); err != nil {
		t.Fatalf("SaveURL: %v", err)
	}

	s2, err := New(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := s2.GetURL("xyz")
	if err != nil {
		t.Fatalf("GetURL after reopen: %v", err)
	}
	if got != "https://persist.example" {
		t.Errorf("want persisted url, got %s", got)
	}
}
