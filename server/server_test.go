package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oliveagle/zvec-go"
)

func adminUsers() []User {
	return []User{
		{Username: "admin", Password: "sha256:8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918"},
		{Username: "viewer", Password: "sha256:d35ca5051b82ffc326a3b0b6574a9a3161dee16b9478a199ee39cd803ce5b799", Readonly: true},
	}
}

func newTestServer(t *testing.T, users []User) *httptest.Server {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Auth.Enabled = true
	cfg.Auth.Users = users
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		srv.mgr.Close()
	})
	return ts
}

func do(t *testing.T, ts *httptest.Server, method, path, user, pass string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func decode(t *testing.T, b []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("decode %q: %v", string(b), err)
	}
}

func mustGet(t *testing.T, s *Server, name string) *zvec.Collection {
	t.Helper()
	c, ok := s.mgr.Get(name)
	if !ok {
		t.Fatalf("collection %q not found", name)
	}
	return c
}

func TestHealthNoAuth(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	code, body := do(t, ts, "GET", "/healthz", "", "", nil)
	if code != http.StatusOK {
		t.Fatalf("health: got %d, want 200: %s", code, body)
	}
	var h healthResponse
	decode(t, body, &h)
	if h.Status != "ok" || !h.AuthEnabled {
		t.Fatalf("unexpected health: %+v", h)
	}
}

func TestAuthRejectsMissingAndBadCreds(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	if code, _ := do(t, ts, "GET", "/api/v1/collections", "", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no creds: got %d, want 401", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/collections", "admin", "wrong", nil); code != http.StatusUnauthorized {
		t.Fatalf("bad creds: got %d, want 401", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/collections", "admin", "admin", nil); code != http.StatusOK {
		t.Fatalf("good creds: got %d, want 200", code)
	}
}

func TestCollectionLifecycle(t *testing.T) {
	ts := newTestServer(t, adminUsers())

	body := createCollectionRequest{
		Name:   "events",
		Fields: []fieldSpec{{Name: "ts", DataType: "INT64"}, {Name: "kind", DataType: "STRING"}},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	if code, _ := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusConflict {
		t.Fatalf("dup create: got %d, want 409", code)
	}

	code, b := do(t, ts, "GET", "/api/v1/collections", "admin", "admin", nil)
	if code != http.StatusOK {
		t.Fatalf("list: got %d", code)
	}
	var lc listCollectionsResponse
	decode(t, b, &lc)
	if lc.Count != 1 || lc.Collections[0].Name != "events" {
		t.Fatalf("unexpected list: %+v", lc)
	}

	up := upsertRequest{Documents: []documentSpec{
		{ID: "e1", Fields: map[string]interface{}{"ts": 1, "kind": "a"}},
		{ID: "e2", Fields: map[string]interface{}{"ts": 2, "kind": "b"}},
	}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/events/documents", "admin", "admin", up); code != http.StatusOK {
		t.Fatalf("upsert: got %d: %s", code, b)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/collections/events/documents/e1", "admin", "admin", nil); code != http.StatusOK {
		t.Fatalf("get doc: got %d", code)
	}
	code, b = do(t, ts, "GET", "/api/v1/collections/events/documents?limit=10", "admin", "admin", nil)
	if code != http.StatusOK {
		t.Fatalf("list docs: got %d", code)
	}
	var ld listDocumentsResponse
	decode(t, b, &ld)
	if ld.Total != 2 || len(ld.Documents) != 2 {
		t.Fatalf("unexpected docs: total=%d n=%d", ld.Total, len(ld.Documents))
	}
	if code, _ := do(t, ts, "DELETE", "/api/v1/collections/events/documents/e1", "admin", "admin", nil); code != http.StatusNoContent {
		t.Fatalf("delete doc: got %d, want 204", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/collections/events/documents/e1", "admin", "admin", nil); code != http.StatusNotFound {
		t.Fatalf("get deleted: got %d, want 404", code)
	}
	if code, _ := do(t, ts, "DELETE", "/api/v1/collections/events", "admin", "admin", nil); code != http.StatusNoContent {
		t.Fatalf("drop: got %d, want 204", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/collections/events", "admin", "admin", nil); code != http.StatusNotFound {
		t.Fatalf("get dropped: got %d, want 404", code)
	}
}

func TestVectorQuerySorted(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	body := createCollectionRequest{
		Name:         "docs",
		VectorFields: []vectorFieldSpec{{Name: "embedding", DataType: "VECTOR_FP32", Dimension: 3, MetricType: "COSINE"}},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	up := upsertRequest{Documents: []documentSpec{
		{ID: "d1", Vectors: map[string][]float32{"embedding": {1.0, 0.0, 0.0}}},
		{ID: "d2", Vectors: map[string][]float32{"embedding": {0.0, 1.0, 0.0}}},
		{ID: "d3", Vectors: map[string][]float32{"embedding": {0.9, 0.1, 0.0}}},
	}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/docs/documents", "admin", "admin", up); code != http.StatusOK {
		t.Fatalf("upsert: got %d: %s", code, b)
	}
	q := queryRequest{Field: "embedding", Vector: []float32{1.0, 0.0, 0.0}, TopK: 3}
	code, b := do(t, ts, "POST", "/api/v1/collections/docs/query", "admin", "admin", q)
	if code != http.StatusOK {
		t.Fatalf("query: got %d: %s", code, b)
	}
	var qr queryResponse
	decode(t, b, &qr)
	if len(qr.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(qr.Results))
	}
	if qr.Results[0].ID != "d1" {
		t.Fatalf("expected d1 first, got %s", qr.Results[0].ID)
	}
	if qr.Results[2].ID != "d2" {
		t.Fatalf("expected d2 last, got %s", qr.Results[2].ID)
	}
}

func TestReadonlyUser(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	if code, _ := do(t, ts, "GET", "/api/v1/collections", "viewer", "viewer", nil); code != http.StatusOK {
		t.Fatalf("viewer read: got %d, want 200", code)
	}
	body := createCollectionRequest{Name: "x", Fields: []fieldSpec{{Name: "f", DataType: "STRING"}}}
	if code, _ := do(t, ts, "POST", "/api/v1/collections", "viewer", "viewer", body); code != http.StatusForbidden {
		t.Fatalf("viewer create: got %d, want 403", code)
	}
}

func TestPersistenceAcrossRestart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Auth.Enabled = true
	cfg.Auth.Users = adminUsers()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	s1, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New#1: %v", err)
	}
	schema := zvec.NewCollectionSchema("persist")
	schema.AddVectorField(zvec.NewVectorSchema("embedding", zvec.DataTypeVectorFP32, 3).WithMetricType(zvec.MetricTypeCOSINE))
	if _, err := s1.mgr.Create("persist", schema); err != nil {
		t.Fatalf("create: %v", err)
	}
	c1 := mustGet(t, s1, "persist")
	docs := []*zvec.Document{
		zvec.NewDocument("d1").SetVector("embedding", []float32{1.0, 0.0, 0.0}),
		zvec.NewDocument("d3").SetVector("embedding", []float32{0.9, 0.1, 0.0}),
	}
	if _, err := c1.UpsertBatch(docs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	s1.mgr.Close()

	s2, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New#2: %v", err)
	}
	defer s2.mgr.Close()
	c2 := mustGet(t, s2, "persist")
	q := zvec.NewVectorQueryByVector("embedding", []float32{1.0, 0.0, 0.0}).WithTopK(2)
	results, err := c2.Query(q, 2, "", false, nil)
	if err != nil {
		t.Fatalf("query after reopen: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results after reopen, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Fatalf("expected d1 first after reopen, got %s", results[0].ID)
	}
}

func TestUIServed(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("UI: got %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("UI content-type: got %q", ct)
	}
	body, _ := io.ReadAll(res.Body)
	if !bytes.Contains(body, []byte("zvec")) {
		t.Fatalf("UI body does not contain 'zvec'")
	}
	// /api unknown path should still be a JSON 404, not the UI.
	res2, err := http.Get(ts.URL + "/api/nope")
	if err != nil {
		t.Fatalf("get /api/nope: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusNotFound {
		t.Fatalf("/api/nope: got %d, want 404", res2.StatusCode)
	}
}
