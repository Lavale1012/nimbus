package cache

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/redis/go-redis/v9"
)

// newTestClient returns a Redis client connected to localhost:6379.
// Tests that call this will skip if Redis is not reachable.
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb, err := NewRedisClient()
	if err != nil {
		t.Skipf("Redis not available, skipping: %v", err)
	}
	t.Cleanup(func() {
		rdb.Del(context.Background(), "user:session")
		rdb.Close()
	})
	return rdb
}

// --- GetAuthToken / SetAuthToken ---

func TestSetAndGetAuthToken(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{{"name": "Home-Box"}}
	if err := SetAuthToken(rdb, 1, "user@example.com", boxes, "test-token"); err != nil {
		t.Fatalf("SetAuthToken: %v", err)
	}

	token, err := GetAuthToken(rdb)
	if err != nil {
		t.Fatalf("GetAuthToken: %v", err)
	}
	if token != "test-token" {
		t.Errorf("expected token %q, got %q", "test-token", token)
	}
}

func TestSetAuthToken_SetsCurrentBoxFromFirstBox(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{{"name": "My-Box"}, {"name": "Other-Box"}}
	if err := SetAuthToken(rdb, 2, "a@b.com", boxes, "tok"); err != nil {
		t.Fatalf("SetAuthToken: %v", err)
	}

	boxName, err := GetBoxName(rdb)
	if err != nil {
		t.Fatalf("GetBoxName: %v", err)
	}
	if boxName != "My-Box" {
		t.Errorf("expected CurrentBox %q, got %q", "My-Box", boxName)
	}
}

func TestSetAuthToken_EmptyBoxList(t *testing.T) {
	rdb := newTestClient(t)

	if err := SetAuthToken(rdb, 3, "a@b.com", nil, "tok"); err != nil {
		t.Fatalf("SetAuthToken: %v", err)
	}

	boxName, err := GetBoxName(rdb)
	if err != nil {
		t.Fatalf("GetBoxName: %v", err)
	}
	if boxName != "" {
		t.Errorf("expected empty CurrentBox, got %q", boxName)
	}
}

func TestGetAuthToken_NoSession(t *testing.T) {
	rdb := newTestClient(t)

	_, err := GetAuthToken(rdb)
	if err == nil {
		t.Fatal("expected error when no session exists, got nil")
	}
}

// --- ClearAuthToken ---

func TestClearAuthToken(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{{"name": "Home-Box"}}
	SetAuthToken(rdb, 1, "user@example.com", boxes, "tok")

	if err := ClearAuthToken(rdb); err != nil {
		t.Fatalf("ClearAuthToken: %v", err)
	}

	exists, err := SessionExists(rdb)
	if err != nil {
		t.Fatalf("SessionExists: %v", err)
	}
	if exists {
		t.Error("expected session to be gone after ClearAuthToken")
	}
}

// --- SessionExists ---

func TestSessionExists_True(t *testing.T) {
	rdb := newTestClient(t)

	SetAuthToken(rdb, 1, "user@example.com", nil, "tok")

	exists, err := SessionExists(rdb)
	if err != nil {
		t.Fatalf("SessionExists: %v", err)
	}
	if !exists {
		t.Error("expected session to exist")
	}
}

func TestSessionExists_False(t *testing.T) {
	rdb := newTestClient(t)

	exists, err := SessionExists(rdb)
	if err != nil {
		t.Fatalf("SessionExists: %v", err)
	}
	if exists {
		t.Error("expected no session")
	}
}

// --- StoreBoxes / BoxExists ---

func TestStoreBoxesAndBoxExists(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{
		{"name": "alpha"},
		{"name": "beta"},
	}
	if err := StoreBoxes(rdb, boxes); err != nil {
		t.Fatalf("StoreBoxes: %v", err)
	}

	tests := []struct {
		name string
		want bool
	}{
		{"alpha", true},
		{"beta", true},
		{"gamma", false},
		{"", false},
	}
	for _, tc := range tests {
		got, err := BoxExists(rdb, tc.name)
		if err != nil {
			t.Errorf("BoxExists(%q): unexpected error: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("BoxExists(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBoxExists_NoBoxesStored(t *testing.T) {
	rdb := newTestClient(t)

	exists, err := BoxExists(rdb, "alpha")
	if err != nil {
		t.Fatalf("BoxExists: %v", err)
	}
	if exists {
		t.Error("expected false when no boxes are stored")
	}
}

func TestBoxExists_InvalidJSON(t *testing.T) {
	rdb := newTestClient(t)

	rdb.HSet(context.Background(), "user:session", "Boxes", "not-valid-json")

	_, err := BoxExists(rdb, "alpha")
	if err == nil {
		t.Error("expected error on invalid JSON, got nil")
	}
}

// --- SetBoxName ---

func TestSetBoxName(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{{"name": "Home-Box"}}
	StoreBoxes(rdb, boxes)

	if err := SetBoxName(rdb, "Home-Box"); err != nil {
		t.Fatalf("SetBoxName: %v", err)
	}

	got, err := GetBoxName(rdb)
	if err != nil {
		t.Fatalf("GetBoxName: %v", err)
	}
	if got != "Home-Box" {
		t.Errorf("expected %q, got %q", "Home-Box", got)
	}
}

func TestSetBoxName_EmptyName(t *testing.T) {
	rdb := newTestClient(t)

	StoreBoxes(rdb, []map[string]any{{"name": "Home-Box"}})

	if err := SetBoxName(rdb, ""); err == nil {
		t.Error("expected error for empty box name, got nil")
	}
}

func TestSetBoxName_NoBoxesField(t *testing.T) {
	rdb := newTestClient(t)

	if err := SetBoxName(rdb, "Home-Box"); err == nil {
		t.Error("expected error when Boxes field doesn't exist, got nil")
	}
}

// --- GetBoxName ---

func TestGetBoxName_NotFound(t *testing.T) {
	rdb := newTestClient(t)

	_, err := GetBoxName(rdb)
	if err == nil {
		t.Error("expected error when CurrentBox not set, got nil")
	}
}

// --- SetCurrentPath / GetCurrentPath ---

func TestSetAndGetCurrentPath(t *testing.T) {
	rdb := newTestClient(t)

	cases := []string{"", "docs", "docs/reports", "a/b/c"}
	for _, path := range cases {
		if err := SetCurrentPath(rdb, path); err != nil {
			t.Fatalf("SetCurrentPath(%q): %v", path, err)
		}
		got, err := GetCurrentPath(rdb)
		if err != nil {
			t.Fatalf("GetCurrentPath after setting %q: %v", path, err)
		}
		if got != path {
			t.Errorf("expected path %q, got %q", path, got)
		}
	}
}

func TestGetCurrentPath_NotFound(t *testing.T) {
	rdb := newTestClient(t)

	_, err := GetCurrentPath(rdb)
	if err == nil {
		t.Error("expected error when CurrentPath not set, got nil")
	}
}

// --- StoreBoxes marshaling ---

func TestStoreBoxes_CorrectJSON(t *testing.T) {
	rdb := newTestClient(t)

	boxes := []map[string]any{{"name": "test", "size": float64(1024)}}
	if err := StoreBoxes(rdb, boxes); err != nil {
		t.Fatalf("StoreBoxes: %v", err)
	}

	raw, err := rdb.HGet(context.Background(), "user:session", "Boxes").Result()
	if err != nil {
		t.Fatalf("HGet Boxes: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("stored value is not valid JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 box, got %d", len(got))
	}
	if got[0]["name"] != "test" {
		t.Errorf("expected name %q, got %v", "test", got[0]["name"])
	}
}
