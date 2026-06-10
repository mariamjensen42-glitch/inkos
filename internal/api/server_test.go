package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestServer(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	srv := NewServer(dir)
	return srv.Engine(), dir
}

func doJSON(t *testing.T, r http.Handler, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rdr = bytes.NewReader(data)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]interface{}{}
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w.Code, out
}

func TestServerHealth(t *testing.T) {
	r, _ := newTestServer(t)
	code, body := doJSON(t, r, "GET", "/api/v1/health", nil)
	if code != 200 || body["status"] != "ok" {
		t.Errorf("health: %d %v", code, body)
	}
}

func TestServerProjectCRUD(t *testing.T) {
	r, _ := newTestServer(t)
	code, body := doJSON(t, r, "GET", "/api/v1/project", nil)
	if code != 200 {
		t.Errorf("GET project: %d", code)
	}
	if body["name"] != "inkos-project" {
		t.Errorf("got %v", body)
	}
	// Set language.
	code, _ = doJSON(t, r, "POST", "/api/v1/project/language", map[string]string{"language": "en"})
	if code != 200 {
		t.Errorf("set language: %d", code)
	}
}

func TestServerBookLifecycle(t *testing.T) {
	r, dir := newTestServer(t)
	// Create book.
	code, body := doJSON(t, r, "POST", "/api/v1/books/create", map[string]interface{}{
		"title": "Sky World", "genre": "xuanhuan", "language": "en",
		"platform": "other", "chapterWordCount": 2500, "targetChapters": 50,
	})
	if code != 200 {
		t.Fatalf("create book: %d %v", code, body)
	}
	id := body["id"].(string)
	if id != "sky-world" {
		t.Errorf("id = %q, want sky-world", id)
	}
	// Get book.
	code, body = doJSON(t, r, "GET", "/api/v1/books/"+id, nil)
	if code != 200 {
		t.Errorf("get book: %d", code)
	}
	// List.
	code, body = doJSON(t, r, "GET", "/api/v1/books", nil)
	if code != 200 {
		t.Errorf("list books: %d", code)
	}
	books := body["books"].([]interface{})
	if len(books) != 1 {
		t.Errorf("list: %v", books)
	}
	// Files exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "books", id, "book.json")); err != nil {
		t.Errorf("book.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "books", id, "story")); err != nil {
		t.Errorf("story dir missing: %v", err)
	}
	// Delete.
	code, _ = doJSON(t, r, "DELETE", "/api/v1/books/"+id, nil)
	if code != 200 {
		t.Errorf("delete: %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "books", id)); !os.IsNotExist(err) {
		t.Errorf("book dir not removed")
	}
}

func TestServerBookIDSafety(t *testing.T) {
	r, _ := newTestServer(t)
	// Leading dot is rejected by util.IsSafeBookID.
	code, _ := doJSON(t, r, "GET", "/api/v1/books/.hidden", nil)
	if code != 400 {
		t.Errorf("expected 400 for unsafe id, got %d", code)
	}
	// Single digit starts with digit; safeBookIDPattern requires letter start.
	code, _ = doJSON(t, r, "GET", "/api/v1/books/123", nil)
	if code != 400 {
		t.Errorf("expected 400 for digit-start, got %d", code)
	}
}

func TestServerChapterWriteRead(t *testing.T) {
	r, _ := newTestServer(t)
	_, body := doJSON(t, r, "POST", "/api/v1/books/create", map[string]interface{}{
		"title": "Alpha", "genre": "xuanhuan", "language": "en",
		"platform": "other", "chapterWordCount": 2500,
	})
	id := body["id"].(string)
	code, _ := doJSON(t, r, "PUT", "/api/v1/books/"+id+"/chapters/1", map[string]interface{}{
		"title": "First", "body": "Once upon a time.", "wordCount": 4, "status": "draft",
	})
	if code != 200 {
		t.Errorf("put chapter: %d", code)
	}
	code, body = doJSON(t, r, "GET", "/api/v1/books/"+id+"/chapters/1", nil)
	if code != 200 {
		t.Errorf("get chapter: %d", code)
	}
	if !strings.Contains(string(mustJSON(body)), "Once upon a time.") {
		t.Errorf("body missing: %v", body)
	}
}

func mustJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func TestServerTruthCRUD(t *testing.T) {
	r, _ := newTestServer(t)
	_, body := doJSON(t, r, "POST", "/api/v1/books/create", map[string]interface{}{
		"title": "Beta", "genre": "xuanhuan", "language": "en",
	})
	id := body["id"].(string)
	code, _ := doJSON(t, r, "PUT", "/api/v1/books/"+id+"/truth/book_rules.md", map[string]string{
		"body": "# rules\n",
	})
	if code != 200 {
		t.Errorf("put truth: %d", code)
	}
	code, body = doJSON(t, r, "GET", "/api/v1/books/"+id+"/truth/book_rules.md", nil)
	if code != 200 || body["body"] != "# rules\n" {
		t.Errorf("get truth: %d %v", code, body)
	}
	code, body = doJSON(t, r, "GET", "/api/v1/books/"+id+"/truth", nil)
	if code != 200 {
		t.Errorf("list truth: %d", code)
	}
	files := body["files"].([]interface{})
	if len(files) != 1 || files[0] != "book_rules.md" {
		t.Errorf("truth list: %v", files)
	}
}

func TestServerServicesConfig(t *testing.T) {
	r, _ := newTestServer(t)
	code, body := doJSON(t, r, "PUT", "/api/v1/services/config", map[string]interface{}{
		"services": map[string]interface{}{
			"moonshot": map[string]interface{}{
				"provider": "custom", "baseUrl": "https://api.moonshot.cn/v1", "model": "kimi",
			},
		},
		"currentService": "moonshot",
		"defaultModel":   "kimi",
	})
	if code != 200 {
		t.Errorf("put services config: %d %v", code, body)
	}
	code, body = doJSON(t, r, "GET", "/api/v1/services", nil)
	if code != 200 {
		t.Errorf("list services: %d", code)
	}
	svcs := body["services"].([]interface{})
	if len(svcs) != 1 {
		t.Errorf("services: %v", svcs)
	}
}

func TestServerSecret(t *testing.T) {
	r, dir := newTestServer(t)
	code, _ := doJSON(t, r, "PUT", "/api/v1/services/openai/secret", map[string]string{"apiKey": "sk-abc"})
	if code != 200 {
		t.Errorf("put secret: %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".inkos", "secrets.json")); err != nil {
		t.Errorf("secrets.json missing: %v", err)
	}
	code, body := doJSON(t, r, "GET", "/api/v1/services/openai/secret", nil)
	if code != 200 || body["apiKey"] != "sk-abc" {
		t.Errorf("get secret: %d %v", code, body)
	}
}

func TestServerDoctor(t *testing.T) {
	r, _ := newTestServer(t)
	code, _ := doJSON(t, r, "GET", "/api/v1/doctor", nil)
	if code != 200 {
		t.Errorf("doctor: %d", code)
	}
}

func TestServerSessions(t *testing.T) {
	r, _ := newTestServer(t)
	code, body := doJSON(t, r, "POST", "/api/v1/sessions", map[string]string{
		"kind": "chat", "title": "Test",
	})
	if code != 200 {
		t.Fatalf("create session: %d", code)
	}
	id := body["sessionId"].(string)
	code, body = doJSON(t, r, "GET", "/api/v1/sessions/"+id, nil)
	if code != 200 {
		t.Errorf("get session: %d", code)
	}
	code, _ = doJSON(t, r, "DELETE", "/api/v1/sessions/"+id, nil)
	if code != 200 {
		t.Errorf("delete: %d", code)
	}
}

func TestServerGenres(t *testing.T) {
	r, _ := newTestServer(t)
	code, body := doJSON(t, r, "GET", "/api/v1/genres", nil)
	if code != 200 {
		t.Errorf("list genres: %d", code)
	}
	g := body["genres"].([]interface{})
	if len(g) < 5 {
		t.Errorf("expected at least 5 genres, got %d", len(g))
	}
}
