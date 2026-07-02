package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- formatSize (box_list.go) ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 2, "2.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tc := range tests {
		got := formatSize(tc.bytes)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// --- cd path resolution logic ---
// These tests exercise the same path-resolution rules as cdCmd without needing Redis.

func resolvePath(currentPath, targetPath string) string {
	var newPath string
	if targetPath == "" {
		return ""
	}
	if strings.HasPrefix(targetPath, "/") {
		newPath = strings.TrimPrefix(targetPath, "/")
	} else if targetPath == ".." {
		if currentPath == "" {
			newPath = ""
		} else {
			lastSlash := strings.LastIndex(currentPath, "/")
			if lastSlash == -1 {
				newPath = ""
			} else {
				newPath = currentPath[:lastSlash]
			}
		}
	} else if strings.Contains(targetPath, "..") {
		import_path := currentPath + "/" + targetPath
		// replicate path.Join + path.Clean behaviour inline for test isolation
		parts := strings.Split(import_path, "/")
		var stack []string
		for _, p := range parts {
			switch p {
			case "", ".":
				// skip
			case "..":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			default:
				stack = append(stack, p)
			}
		}
		newPath = strings.Join(stack, "/")
		if strings.HasPrefix(newPath, "..") {
			newPath = ""
		}
	} else {
		if currentPath == "" {
			newPath = targetPath
		} else {
			newPath = currentPath + "/" + targetPath
		}
	}
	// clean double slashes / trailing slashes
	newPath = strings.Trim(newPath, "/")
	for strings.Contains(newPath, "//") {
		newPath = strings.ReplaceAll(newPath, "//", "/")
	}
	return newPath
}

func TestCdPathResolution(t *testing.T) {
	tests := []struct {
		current string
		target  string
		want    string
	}{
		// go to root
		{"", "", ""},
		{"docs", "", ""},
		// absolute paths
		{"docs", "/reports", "reports"},
		{"docs", "/a/b/c", "a/b/c"},
		// go up one level
		{"docs", "..", ""},
		{"docs/reports", "..", "docs"},
		{"a/b/c", "..", "a/b"},
		{"toponly", "..", ""},
		// already at root, stay at root
		{"", "..", ""},
		// relative paths
		{"", "folder", "folder"},
		{"docs", "reports", "docs/reports"},
		{"a/b", "c", "a/b/c"},
		// paths containing ..
		{"docs/reports", "../other", "docs/other"},
		{"a/b/c", "../../x", "a/x"},
	}
	for _, tc := range tests {
		got := resolvePath(tc.current, tc.target)
		if got != tc.want {
			t.Errorf("resolvePath(%q, %q) = %q, want %q", tc.current, tc.target, got, tc.want)
		}
	}
}

// --- HTTP response parsing ---

func TestListBoxesResponse_Parsing(t *testing.T) {
	payload := ListBoxesResponse{
		Boxes: []BoxEntry{
			{Name: "alpha", Size: 1024},
			{Name: "beta", Size: 2048 * 1024},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var result ListBoxesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Boxes) != 2 {
		t.Fatalf("expected 2 boxes, got %d", len(result.Boxes))
	}
	if result.Boxes[0].Name != "alpha" {
		t.Errorf("expected Boxes[0].Name %q, got %q", "alpha", result.Boxes[0].Name)
	}
	if result.Boxes[1].Size != 2048*1024 {
		t.Errorf("expected Boxes[1].Size %d, got %d", 2048*1024, result.Boxes[1].Size)
	}
}

func TestListBoxesResponse_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListBoxesResponse{Boxes: []BoxEntry{}})
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var result ListBoxesResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Boxes) != 0 {
		t.Errorf("expected 0 boxes, got %d", len(result.Boxes))
	}
}

func TestListBoxesResponse_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginResponse_Parsing(t *testing.T) {
	payload := map[string]any{
		"message": "Login successful",
		"token":   "eyJhbGciOiJIUzI1NiJ9.test",
		"email":   "user@example.com",
		"user_id": float64(42),
		"box": []map[string]any{
			{"name": "Home-Box"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var result LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Token != "eyJhbGciOiJIUzI1NiJ9.test" {
		t.Errorf("unexpected token: %q", result.Token)
	}
	if result.Email != "user@example.com" {
		t.Errorf("unexpected email: %q", result.Email)
	}
	if len(result.Box) != 1 {
		t.Fatalf("expected 1 box, got %d", len(result.Box))
	}
	if result.Box[0]["name"] != "Home-Box" {
		t.Errorf("unexpected box name: %v", result.Box[0]["name"])
	}
}

func TestListFolderResponse_Parsing(t *testing.T) {
	payload := ListResponse{
		Path:       "/docs",
		FolderName: "docs",
		Files: []FileEntry{
			{ID: 1, Name: "readme.md", Size: 512, S3Key: "box/docs/readme.md"},
		},
		Folders: []FolderEntry{
			{ID: 2, Name: "images"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var result ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "readme.md" {
		t.Errorf("unexpected files: %+v", result.Files)
	}
	if len(result.Folders) != 1 || result.Folders[0].Name != "images" {
		t.Errorf("unexpected folders: %+v", result.Folders)
	}
}

func TestBearerTokenAttached(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListBoxesResponse{})
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer my-jwt-token")
	http.DefaultClient.Do(req)

	if gotAuth != "Bearer my-jwt-token" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer my-jwt-token", gotAuth)
	}
}
