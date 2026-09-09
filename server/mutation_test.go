package server

// Focused tests that close the test gaps revealed by mutation testing
// (gomut) in the server package: boundary conditions in config defaults,
// error propagation in the manager, request-body validation, and auth
// semantics.
//
// Mutants intentionally not chased (equivalent or unobservable):
//   - getCollection's (nil, false) return values: both call sites read
//     only the boolean, so the replaced pointer is never used.
//   - create handler field checks "f.Name == "" || !f.DataType.IsScalar()"
//     (and the vector-field twin): the swapped "&&" still 400s, because
//     the schema-level Validate rejects the same input afterwards.
//   - query handler "req.Field == "" || len(req.Vector) == 0": the swapped
//     "&&" still 400s via the root VectorQuery.Validate (empty field /
//     missing vector are both rejected there).
//   - get-document "err != nil || doc == nil": Collection.Get never
//     returns exactly one non-nil of (doc, err), so the swap is
//     indistinguishable.
//   - manager.Create's duplicate-name return: callers inspect the error
//     before touching the collection value, so the replaced first value is
//     dead.
//   - toDocument's make(..., len+1) capacity constant: an allocation hint
//     only, unobservable through the API.
//   - handleQuery's "results == nil" guard: the root Query always returns a
//     non-nil slice, so the guard is dead code.
//   - list-docs "limit > 1000" vs ">= 1000": clamping to 1000 at
//     limit == 1000 is a no-op, so the boundary mutant is equivalent.
//   - NewCollectionManager's 0o755 mode constant: modes depend on umask and
//     filesystem and do not affect behavior here.
//   - Drop's Destroy-failure return: forcing a Destroy failure needs
//     privileges (e.g. a read-only filesystem) and cannot be injected from
//     a normal test user.
//   - types.go info() "err == nil && st != nil" -> "||": Stats never
//     returns a non-nil stats alongside an error, so the swap is
//     indistinguishable.
//   - server.go New: the zvec.Init(nil) error return is dead (Init(nil)
//     performs no file operations and never fails); the OpenExisting
//     error check and the plain-text-password warning are log-only (tests
//     discard the logger output); the auth-configure error return's first
//     value is dead (callers inspect the error first).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oliveagle/zvec-go"
)

// ensureInit initializes the zvec client once per test process (required
// before creating collections directly, outside the Server path).
func ensureInit(t *testing.T) {
	t.Helper()
	if err := zvec.Init(nil); err != nil {
		t.Fatalf("zvec.Init: %v", err)
	}
}

// newTestServerWithDir is like newTestServer but lets the test reach the
// on-disk data directory (needed to plant files that force manager errors).
func newTestServerWithDir(t *testing.T, users []User, dataDir string) (*httptest.Server, *Server) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Storage.DataDir = dataDir
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
	return ts, srv
}

// rawRequest posts a pre-serialized body (bypassing do()'s json.Marshal so
// malformed and oversized bodies can be exercised).
func rawRequest(t *testing.T, ts *httptest.Server, method, path, user, pass, raw string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// ---- config.go ----

// TestDefaultConfigExact pins every default returned by DefaultConfig,
// killing the zero-struct and constant mutants.
func TestDefaultConfigExact(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want 127.0.0.1:8080", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeoutSeconds != 30 || cfg.Server.WriteTimeoutSeconds != 30 || cfg.Server.ShutdownTimeoutSeconds != 10 {
		t.Errorf("timeouts = %d/%d/%d, want 30/30/10",
			cfg.Server.ReadTimeoutSeconds, cfg.Server.WriteTimeoutSeconds, cfg.Server.ShutdownTimeoutSeconds)
	}
	if cfg.Storage.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.Storage.DataDir)
	}
	if cfg.Auth.Enabled {
		t.Error("Auth.Enabled = true, want false (default is unauthenticated; New warns about it)")
	}
}

// TestFillDefaultsBoundaries covers fillDefaults at the boundaries: zero
// values must be filled, values of exactly 1 must be preserved (kills
// "<= 0" -> "<= 1" and negated mutants), and custom values must survive.
func TestFillDefaultsBoundaries(t *testing.T) {
	c := &Config{}
	c.fillDefaults()
	if c.Server.Addr != "127.0.0.1:8080" || c.Server.ReadTimeoutSeconds != 30 ||
		c.Server.WriteTimeoutSeconds != 30 || c.Server.ShutdownTimeoutSeconds != 10 ||
		c.Storage.DataDir != "./data" {
		t.Fatalf("zero config not filled: %+v", c)
	}

	ones := &Config{
		Server:  ServerConfig{Addr: "0.0.0.0:1", ReadTimeoutSeconds: 1, WriteTimeoutSeconds: 1, ShutdownTimeoutSeconds: 1},
		Storage: StorageConfig{DataDir: "/tmp/kept"},
	}
	ones.fillDefaults()
	if ones.Server.Addr != "0.0.0.0:1" || ones.Server.ReadTimeoutSeconds != 1 ||
		ones.Server.WriteTimeoutSeconds != 1 || ones.Server.ShutdownTimeoutSeconds != 1 ||
		ones.Storage.DataDir != "/tmp/kept" {
		t.Fatalf("value-1 config altered by fillDefaults: %+v", ones)
	}

	neg := &Config{
		Server:  ServerConfig{ReadTimeoutSeconds: -5, WriteTimeoutSeconds: -5, ShutdownTimeoutSeconds: -5},
		Storage: StorageConfig{DataDir: ""},
	}
	neg.fillDefaults()
	if neg.Server.ReadTimeoutSeconds != 30 || neg.Server.WriteTimeoutSeconds != 30 ||
		neg.Server.ShutdownTimeoutSeconds != 10 || neg.Storage.DataDir != "./data" {
		t.Fatalf("negative values not replaced: %+v", neg)
	}
}

// TestTimeoutDurations pins the exact duration arithmetic, killing the
// "* time.Second" -> "/ time.Second" and zero-return mutants.
func TestTimeoutDurations(t *testing.T) {
	c := &Config{Server: ServerConfig{ReadTimeoutSeconds: 5, WriteTimeoutSeconds: 6, ShutdownTimeoutSeconds: 7}}
	if c.ReadTimeout() != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", c.ReadTimeout())
	}
	if c.WriteTimeout() != 6*time.Second {
		t.Errorf("WriteTimeout = %v, want 6s", c.WriteTimeout())
	}
	if c.ShutdownTimeout() != 7*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 7s", c.ShutdownTimeout())
	}
	d := DefaultConfig()
	if d.ReadTimeout() != 30*time.Second || d.WriteTimeout() != 30*time.Second || d.ShutdownTimeout() != 10*time.Second {
		t.Errorf("default timeouts = %v/%v/%v, want 30s/30s/10s",
			d.ReadTimeout(), d.WriteTimeout(), d.ShutdownTimeout())
	}
}

// TestLoadConfigErrorsAndDefaults exercises every LoadConfig error branch
// (missing file, corrupt JSON, trailing data) and the partial-file success
// path with defaults filled in.
func TestLoadConfigErrorsAndDefaults(t *testing.T) {
	dir := t.TempDir()

	if cfg, err := LoadConfig(filepath.Join(dir, "nope.json")); err == nil || cfg != nil ||
		!strings.Contains(err.Error(), "read config") {
		t.Errorf("missing file: %v, %v; want (nil, read-config failure)", cfg, err)
	}

	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if cfg, err := LoadConfig(write("bad.json", "{ not json")); err == nil || cfg != nil ||
		!strings.Contains(err.Error(), "parse config") {
		t.Errorf("corrupt JSON: %v, %v; want (nil, parse failure)", cfg, err)
	}
	if cfg, err := LoadConfig(write("trail.json", `{"server":{"addr":"1.2.3.4:1"}}EXTRA`)); err == nil || cfg != nil ||
		!strings.Contains(err.Error(), "trailing data") {
		t.Errorf("trailing data: %v, %v; want (nil, trailing-data failure)", cfg, err)
	}

	cfg, err := LoadConfig(write("partial.json", `{"server":{"addr":"127.0.0.1:1234"},"auth":{"enabled":false}}`))
	if err != nil {
		t.Fatalf("partial config: %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:1234" || cfg.Server.ReadTimeoutSeconds != 30 ||
		cfg.Server.WriteTimeoutSeconds != 30 || cfg.Server.ShutdownTimeoutSeconds != 10 ||
		cfg.Storage.DataDir != "./data" || cfg.Auth.Enabled {
		t.Errorf("partial config not filled correctly: %+v", cfg)
	}
}

// ---- manager.go ----

// TestValidateCollectionName pins both sides of the regex predicate.
func TestValidateCollectionName(t *testing.T) {
	for _, ok := range []struct {
		name string
		want bool
	}{
		{"abc", true},
		{"A-b_c9", true},
		{"9x", true},
		{"", false},
		{"bad name", false},
		{"-abc", false},
		{"a/b", false},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
	} {
		if got := ValidateCollectionName(ok.name); got != ok.want {
			t.Errorf("ValidateCollectionName(%q) = %v, want %v", ok.name, got, ok.want)
		}
	}
}

// TestNewCollectionManagerDefaultsAndErrors covers the "" dataDir default,
// the nil-logger default, logger preservation, and MkdirAll failure.
func TestNewCollectionManagerDefaultsAndErrors(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	m, err := NewCollectionManager("", nil)
	if err != nil || m == nil {
		t.Fatalf("NewCollectionManager(\"\") = %v, %v", m, err)
	}
	if m.dataDir != "./data" {
		t.Errorf("dataDir = %q, want ./data", m.dataDir)
	}
	if m.log == nil {
		t.Error("log = nil, want slog.Default()")
	}
	if st, err := os.Stat("./data"); err != nil || !st.IsDir() {
		t.Errorf("./data not created: %v", err)
	}
	m.Close()

	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	m2, err := NewCollectionManager(t.TempDir(), lg)
	if err != nil || m2 == nil || m2.log != lg {
		t.Fatalf("NewCollectionManager(dir, lg) = %v, %v; want the provided logger kept", m2, err)
	}
	m2.Close()

	base := t.TempDir()
	f := filepath.Join(base, "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, err := NewCollectionManager(filepath.Join(f, "sub"), nil); m != nil || err == nil {
		t.Errorf("NewCollectionManager under a file = %v, %v; want (nil, err)", m, err)
	}
}

// TestOpenExistingScenarios covers OpenExisting at startup: empty dir,
// skipped non-collection entries, real collection reload, missing dir, and
// a data dir that is a regular file.
func TestOpenExistingScenarios(t *testing.T) {
	ensureInit(t)
	base := t.TempDir()
	m, err := NewCollectionManager(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	if err := m.OpenExisting(); err != nil {
		t.Fatalf("empty dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "notacoll"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.OpenExisting(); err != nil || m.Count() != 0 {
		t.Fatalf("non-collection dir: count=%d err=%v, want 0/nil", m.Count(), err)
	}

	schema := zvec.NewCollectionSchema("reopen")
	schema.AddField(zvec.NewFieldSchema("n", zvec.DataTypeInt64))
	if _, err := m.Create("reopen", schema); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m2, err := NewCollectionManager(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	if err := m2.OpenExisting(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c, ok := m2.Get("reopen"); !ok || c == nil {
		t.Fatalf("reopened collection missing: ok=%v", ok)
	}

	// A deleted data dir is not an error (fresh install).
	fresh := t.TempDir()
	sub := filepath.Join(fresh, "empty")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m3, err := NewCollectionManager(sub, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m3.Close()
	if err := os.Remove(sub); err != nil {
		t.Fatal(err)
	}
	if err := m3.OpenExisting(); err != nil {
		t.Errorf("removed data dir: err = %v, want nil (IsNotExist)", err)
	}
	// A data dir that is a regular file must surface the ReadDir error.
	if err := os.WriteFile(sub, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m3.OpenExisting(); err == nil {
		t.Error("data dir is a regular file: err = nil, want non-nil")
	}
}

// TestManagerCreateErrorsAndSuccess covers every manager.Create branch:
// invalid name, nil schema, invalid schema, on-disk conflict, duplicate
// name, and success (with pointer identity between Create and Get).
func TestManagerCreateErrorsAndSuccess(t *testing.T) {
	ensureInit(t)
	m, err := NewCollectionManager(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	good := zvec.NewCollectionSchema("good")
	good.AddField(zvec.NewFieldSchema("n", zvec.DataTypeInt64))

	if c, err := m.Create("bad name!", good); c != nil || err == nil {
		t.Errorf("invalid name: %v, %v; want (nil, err)", c, err)
	}
	if c, err := m.Create("nilschema", nil); c != nil || err == nil {
		t.Errorf("nil schema: %v, %v; want (nil, err)", c, err)
	}
	bad := zvec.NewCollectionSchema("x")
	bad.AddField(zvec.NewFieldSchema("f", zvec.DataTypeInt64))
	bad.AddField(zvec.NewFieldSchema("f", zvec.DataTypeInt64))
	if c, err := m.Create("badschema", bad); c != nil || err == nil {
		t.Errorf("invalid schema: %v, %v; want (nil, err)", c, err)
	}
	// A regular file squatting on the collection path must fail creation.
	squatter := filepath.Join(m.dataDir, "fileblock")
	if err := os.WriteFile(squatter, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c, err := m.Create("fileblock", good); c != nil || err == nil {
		t.Errorf("path blocked by file: %v, %v; want (nil, err)", c, err)
	}

	coll, err := m.Create("good", good)
	if err != nil || coll == nil {
		t.Fatalf("Create(good) = %v, %v", coll, err)
	}
	if got, ok := m.Get("good"); !ok || got != coll {
		t.Fatalf("Get(good) = %v, %v; want the exact pointer returned by Create", got, ok)
	}
	if _, err := m.Create("good", good); !errors.Is(err, errCollectionExists) {
		t.Errorf("duplicate create: err = %v, want errCollectionExists", err)
	}
}

// TestManagerGetDropCount covers Get miss/hit, Drop of a missing
// collection, and Count.
func TestManagerGetDropCount(t *testing.T) {
	ensureInit(t)
	m, err := NewCollectionManager(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)

	if c, ok := m.Get("nope"); ok || c != nil {
		t.Fatalf("Get(missing) = %v, %v; want (nil, false)", c, ok)
	}
	schema := zvec.NewCollectionSchema("c1")
	schema.AddField(zvec.NewFieldSchema("n", zvec.DataTypeInt64))
	coll, err := m.Create("c1", schema)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got, ok := m.Get("c1"); !ok || got != coll {
		t.Fatalf("Get(c1) = %v, %v; want the created collection", got, ok)
	}
	if m.Count() != 1 {
		t.Errorf("Count = %d, want 1", m.Count())
	}
	if err := m.Drop("nope"); !errors.Is(err, errCollectionNotFound) {
		t.Errorf("Drop(missing): err = %v, want errCollectionNotFound", err)
	}
	if err := m.Drop("c1"); err != nil {
		t.Errorf("Drop(c1): %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("Count after drop = %d, want 0", m.Count())
	}
}

// TestManagerInfo kills the types.go info() branch mutants (schema
// description, stats doc count and size) via a direct call.
func TestManagerInfo(t *testing.T) {
	ensureInit(t)
	m, err := NewCollectionManager(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	schema := zvec.NewCollectionSchema("infos")
	schema.Description = "the info collection"
	schema.AddField(zvec.NewFieldSchema("n", zvec.DataTypeInt64))
	if _, err := m.Create("infos", schema); err != nil {
		t.Fatalf("Create: %v", err)
	}
	c, ok := m.Get("infos")
	if !ok {
		t.Fatal("Get(infos): not found")
	}
	if _, err := c.UpsertBatch([]*zvec.Document{zvec.NewDocument("i1").SetField("n", int64(7))}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	info := m.info("infos", c)
	if info.Name != "infos" || info.Description != "the info collection" {
		t.Errorf("info = %+v, want name/description set", info)
	}
	if info.DocCount != 1 || info.SizeBytes <= 0 {
		t.Errorf("info.DocCount/SizeBytes = %d/%d, want 1/>0", info.DocCount, info.SizeBytes)
	}
	if info.Schema == nil || info.Schema.Description != "the info collection" {
		t.Errorf("info.Schema = %+v, want schema with description", info.Schema)
	}
}

// ---- handlers.go ----

// TestDecodeBodyErrorsViaHTTP exercises decodeBody's error branches through
// the real HTTP path: malformed JSON and trailing garbage.
func TestDecodeBodyErrorsViaHTTP(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	if code, b := rawRequest(t, ts, "POST", "/api/v1/collections", "admin", "admin", "{"); code != http.StatusBadRequest {
		t.Errorf("malformed body: got %d, want 400: %s", code, b)
	}
	if code, b := rawRequest(t, ts, "POST", "/api/v1/collections", "admin", "admin", `{"name":"x"}GARBAGE`); code != http.StatusBadRequest {
		t.Errorf("trailing garbage: got %d, want 400: %s", code, b)
	}
	// A type error on a later field of a complete object leaves the earlier
	// fields decoded; decodeBody must not swallow that error (its return-
	// err -> return-nil mutant would let this request through, because the
	// document is complete and dec.More() is false).
	if code, b := rawRequest(t, ts, "POST", "/api/v1/collections", "admin", "admin",
		`{"name":"mutkill","fields":[{"name":"f","data_type":"STRING"}],"description":123}`); code != http.StatusBadRequest {
		t.Errorf("later-field type error: got %d, want 400: %.200s", code, b)
	}
}

// TestReadonlyWriteSideEffect pins requireWritable's return value: a
// rejected readonly write must not mutate server state (the
// return-false -> return-true mutant writes the 403 but then proceeds to
// create the collection).
func TestReadonlyWriteSideEffect(t *testing.T) {
	ts, _ := newTestServerWithDir(t, adminUsers(), t.TempDir())
	body := createCollectionRequest{Name: "nope403", Fields: []fieldSpec{{Name: "n", DataType: "INT64"}}}
	if code, _ := do(t, ts, "POST", "/api/v1/collections", "viewer", "viewer", body); code != http.StatusForbidden {
		t.Fatalf("readonly create: got %d, want 403", code)
	}
	code, b := do(t, ts, "GET", "/api/v1/collections", "admin", "admin", nil)
	if code != http.StatusOK {
		t.Fatalf("list: got %d: %s", code, b)
	}
	var lc listCollectionsResponse
	decode(t, b, &lc)
	if lc.Count != 0 {
		t.Errorf("readonly create leaked into server state: collections = %+v, want none", lc.Collections)
	}
}

// TestBodySizeLimitViaHTTP pins the 10 MiB request-body limit: a ~10.5 MiB
// body must be rejected (kills "10 -> 11" and "20 -> 21" constant mutants
// and the "10 << 20" -> "10 >> 20" rewrite via the small-body control).
func TestBodySizeLimitViaHTTP(t *testing.T) {
	ts, _ := newTestServerWithDir(t, adminUsers(), t.TempDir())

	// Small valid body must go through (kills a zeroed limit).
	body := createCollectionRequest{Name: "small", Fields: []fieldSpec{{Name: "n", DataType: "INT64"}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("small valid body: got %d, want 201: %s", code, b)
	}

	// A ~10.5 MiB valid JSON document must exceed the 10 MiB limit.
	pad := strings.Repeat("a", 10*1024*1024+512*1024)
	if code, b := rawRequest(t, ts, "POST", "/api/v1/collections", "admin", "admin",
		`{"name":"bigcoll","description":"`+pad+`"}`); code != http.StatusBadRequest {
		t.Errorf("10.5 MiB body: got %d, want 400 (too large): %.200s", code, b)
	}
}

// TestCreateCollectionValidationViaHTTP covers the create/get/drop handler
// branches: invalid path name, field validation, metric round-trip,
// schema-level failure (dimension 0), manager-level failure (squatting
// file -> 500), duplicate -> 409, and drop-missing -> 404.
func TestCreateCollectionValidationViaHTTP(t *testing.T) {
	dataDir := t.TempDir()
	ts, _ := newTestServerWithDir(t, adminUsers(), dataDir)

	if code, _ := do(t, ts, "GET", "/api/v1/collections/bad%20name", "admin", "admin", nil); code != http.StatusBadRequest {
		t.Errorf("invalid path name: got %d, want 400", code)
	}

	vf := vectorFieldSpec{Name: "emb", DataType: "VECTOR_FP32", Dimension: 3, MetricType: "IP"}
	body := createCollectionRequest{
		Name:         "metricc",
		Description:  "metric test",
		Fields:       []fieldSpec{{Name: "n", DataType: "INT64"}},
		VectorFields: []vectorFieldSpec{vf},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	up := upsertRequest{Document: &documentSpec{
		ID:      "m1",
		Fields:  map[string]interface{}{"n": 1},
		Vectors: map[string][]float32{"emb": {1, 0, 0}},
	}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/metricc/documents", "admin", "admin", up); code != http.StatusOK {
		t.Fatalf("upsert: got %d: %s", code, b)
	}
	code, b := do(t, ts, "GET", "/api/v1/collections/metricc", "admin", "admin", nil)
	if code != http.StatusOK {
		t.Fatalf("get collection: got %d: %s", code, b)
	}
	var info CollectionInfo
	decode(t, b, &info)
	if info.Description != "metric test" || info.DocCount != 1 {
		t.Errorf("info = %+v, want description and DocCount 1", info)
	}
	if info.Schema == nil || len(info.Schema.VectorFields) != 1 ||
		info.Schema.VectorFields[0].MetricType != "IP" {
		t.Errorf("schema = %+v, want vector field with metric IP", info.Schema)
	}

	cases := []struct {
		name string
		req  createCollectionRequest
	}{
		{"empty field name", createCollectionRequest{Name: "ef", Fields: []fieldSpec{{Name: "", DataType: "STRING"}}}},
		{"empty vector field name", createCollectionRequest{Name: "ev", VectorFields: []vectorFieldSpec{{Name: "", DataType: "VECTOR_FP32", Dimension: 2}}}},
		{"non-scalar field", createCollectionRequest{Name: "ns", Fields: []fieldSpec{{Name: "v", DataType: "VECTOR_FP32"}}}},
		{"zero dimension", createCollectionRequest{Name: "d0", VectorFields: []vectorFieldSpec{{Name: "e", DataType: "VECTOR_FP32", Dimension: 0}}}},
	}
	for _, tc := range cases {
		if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", tc.req); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400: %s", tc.name, code, b)
		}
	}

	// A regular file squatting on the requested collection path makes the
	// manager-level Create fail -> 500 (not the 409 exists path).
	if err := os.WriteFile(filepath.Join(dataDir, "squatter"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body5 := createCollectionRequest{Name: "squatter", Fields: []fieldSpec{{Name: "n", DataType: "INT64"}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body5); code != http.StatusInternalServerError {
		t.Errorf("squatting file: got %d, want 500: %s", code, b)
	}

	if code, _ := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusConflict {
		t.Errorf("duplicate create: got %d, want 409", code)
	}
	if code, _ := do(t, ts, "DELETE", "/api/v1/collections/ghostcoll", "admin", "admin", nil); code != http.StatusNotFound {
		t.Errorf("drop missing: got %d, want 404", code)
	}
}

// TestListDocumentsLimits pins every limit-clamping branch in
// handleListDocuments with a 1001-document collection.
func TestListDocumentsLimits(t *testing.T) {
	ts, _ := newTestServerWithDir(t, adminUsers(), t.TempDir())
	body := createCollectionRequest{Name: "biglist", Fields: []fieldSpec{{Name: "n", DataType: "INT64"}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	docs := make([]documentSpec, 0, 1001)
	for i := 0; i < 1001; i++ {
		docs = append(docs, documentSpec{ID: "d" + pad4(i), Fields: map[string]interface{}{"n": i}})
	}
	up := upsertRequest{Documents: docs}
	if code, b := do(t, ts, "POST", "/api/v1/collections/biglist/documents", "admin", "admin", up); code != http.StatusOK {
		t.Fatalf("upsert 1001 docs: got %d: %s", code, b)
	}

	for _, tc := range []struct {
		limit string
		want  int
	}{
		{"0", 100}, // default limit
		{"1", 1},   // kills "<= 0" -> "<= 1", negate, and boundary
		{"50", 50}, // kills "limit > 1000" negation (would clamp to 1000)
		{"1000", 1000},
		{"1001", 1000}, // kills "1000 -> 1001" on both the compare and the assignment
	} {
		code, b := do(t, ts, "GET", "/api/v1/collections/biglist/documents?limit="+tc.limit, "admin", "admin", nil)
		if code != http.StatusOK {
			t.Fatalf("limit=%s: got %d: %s", tc.limit, code, b)
		}
		var ld listDocumentsResponse
		decode(t, b, &ld)
		if ld.Total != 1001 {
			t.Errorf("limit=%s: total = %d, want 1001", tc.limit, ld.Total)
		}
		if len(ld.Documents) != tc.want {
			t.Errorf("limit=%s: got %d docs, want %d", tc.limit, len(ld.Documents), tc.want)
		}
	}
}

// TestUpsertValidationAndContent covers upsert request validation (empty
// body, invalid single-doc id), full document content round-trip
// (fields/vectors/metadata), and a persist failure surfaced as 500.
func TestUpsertValidationAndContent(t *testing.T) {
	ts, srv := newTestServerWithDir(t, adminUsers(), t.TempDir())
	body := createCollectionRequest{
		Name:         "upv",
		Fields:       []fieldSpec{{Name: "n", DataType: "INT64"}},
		VectorFields: []vectorFieldSpec{{Name: "emb", DataType: "VECTOR_FP32", Dimension: 2}},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}

	// Empty body: neither "document" nor "documents".
	if code, b := do(t, ts, "POST", "/api/v1/collections/upv/documents", "admin", "admin", map[string]string{}); code != http.StatusBadRequest {
		t.Errorf("empty body: got %d, want 400: %s", code, b)
	}
	// Invalid id in the single-document form.
	bad := upsertRequest{Document: &documentSpec{ID: "../evil", Fields: map[string]interface{}{"n": 1}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/upv/documents", "admin", "admin", bad); code != http.StatusBadRequest {
		t.Errorf("invalid id: got %d, want 400: %s", code, b)
	}

	full := upsertRequest{Document: &documentSpec{
		ID:       "full1",
		Fields:   map[string]interface{}{"n": 42, "s": "hello"},
		Vectors:  map[string][]float32{"emb": {1.0, 0.5}},
		Metadata: map[string]interface{}{"src": "mutation", "seq": 3},
	}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/upv/documents", "admin", "admin", full); code != http.StatusOK {
		t.Fatalf("upsert full doc: got %d: %s", code, b)
	}
	code, b := do(t, ts, "GET", "/api/v1/collections/upv/documents/full1", "admin", "admin", nil)
	if code != http.StatusOK {
		t.Fatalf("get doc: got %d: %s", code, b)
	}
	var got documentSpec
	decode(t, b, &got)
	if !numEqual(got.Fields["n"], 42) || got.Fields["s"] != "hello" {
		t.Errorf("fields = %v, want n=42 s=hello", got.Fields)
	}
	if v := got.Vectors["emb"]; len(v) != 2 || v[0] != 1.0 || v[1] != 0.5 {
		t.Errorf("vectors = %v, want [1 0.5]", v)
	}
	if got.Metadata["src"] != "mutation" || !numEqual(got.Metadata["seq"], 3) {
		t.Errorf("metadata = %v, want src=mutation seq=3", got.Metadata)
	}

	// Block the on-disk target of a fresh doc id: UpsertBatch must fail and
	// the handler must report 500 (kills the "err == nil" ReturnVals swap).
	block := filepath.Join(srv.cfg.Storage.DataDir, "upv", "docs", "blocked.json")
	if err := os.MkdirAll(block, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(block, "inner"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	blockDoc := upsertRequest{Document: &documentSpec{ID: "blocked", Fields: map[string]interface{}{"n": 9}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/upv/documents", "admin", "admin", blockDoc); code != http.StatusInternalServerError {
		t.Errorf("blocked persist: got %d, want 500: %s", code, b)
	}
}

// TestGetDocumentNeverExisted kills the "||" -> "&&" swap in
// handleGetDocument's not-found check for a doc that was never written.
func TestGetDocumentNeverExisted(t *testing.T) {
	ts, _ := newTestServerWithDir(t, adminUsers(), t.TempDir())
	body := createCollectionRequest{Name: "ghostc", Fields: []fieldSpec{{Name: "n", DataType: "INT64"}}}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	if code, b := do(t, ts, "GET", "/api/v1/collections/ghostc/documents/never", "admin", "admin", nil); code != http.StatusNotFound {
		t.Errorf("get never-existed doc: got %d, want 404: %s", code, b)
	}
}

// TestQueryValidationAndTopK covers query request validation (missing
// field/vector, 1-element vector) and the topk clamping branches with a
// 12-document collection.
func TestQueryValidationAndTopK(t *testing.T) {
	ts, _ := newTestServerWithDir(t, adminUsers(), t.TempDir())
	body := createCollectionRequest{
		Name:         "qv",
		VectorFields: []vectorFieldSpec{{Name: "emb", DataType: "VECTOR_FP32", Dimension: 3, MetricType: "IP"}},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", body); code != http.StatusCreated {
		t.Fatalf("create: got %d: %s", code, b)
	}
	docs := make([]documentSpec, 0, 12)
	for i := 0; i < 12; i++ {
		docs = append(docs, documentSpec{ID: "q" + pad4(i), Vectors: map[string][]float32{
			"emb": {float32(i) * 0.1, 1.0, 0.2},
		}})
	}
	up := upsertRequest{Documents: docs}
	if code, b := do(t, ts, "POST", "/api/v1/collections/qv/documents", "admin", "admin", up); code != http.StatusOK {
		t.Fatalf("upsert: got %d: %s", code, b)
	}

	// Missing vector.
	if code, b := do(t, ts, "POST", "/api/v1/collections/qv/query", "admin", "admin",
		queryRequest{Field: "emb"}); code != http.StatusBadRequest {
		t.Errorf("missing vector: got %d, want 400: %s", code, b)
	}
	// Missing field.
	if code, b := do(t, ts, "POST", "/api/v1/collections/qv/query", "admin", "admin",
		queryRequest{Vector: []float32{1, 0, 0}}); code != http.StatusBadRequest {
		t.Errorf("missing field: got %d, want 400: %s", code, b)
	}

	// topk=0 must mean the default 10 (kills "<= 0" -> "<", negate, and
	// 10 -> 11); topk=1 must not be clamped (kills 0 -> 1).
	for _, tc := range []struct {
		topk int
		want int
	}{
		{0, 10},
		{1, 1},
		{12, 12},
	} {
		code, b := do(t, ts, "POST", "/api/v1/collections/qv/query", "admin", "admin",
			queryRequest{Field: "emb", Vector: []float32{1.1, 1.0, 0.2}, TopK: tc.topk})
		if code != http.StatusOK {
			t.Fatalf("query topk=%d: got %d: %s", tc.topk, code, b)
		}
		var qr queryResponse
		decode(t, b, &qr)
		if len(qr.Results) != tc.want {
			t.Errorf("topk=%d: got %d results, want %d", tc.topk, len(qr.Results), tc.want)
		}
	}

	// A one-element query vector is valid (kills the "len == 0" ->
	// "len == 1" constant mutant) against a 1-dim collection.
	d1 := createCollectionRequest{
		Name:         "q1",
		VectorFields: []vectorFieldSpec{{Name: "e", DataType: "VECTOR_FP32", Dimension: 1}},
	}
	if code, b := do(t, ts, "POST", "/api/v1/collections", "admin", "admin", d1); code != http.StatusCreated {
		t.Fatalf("create q1: got %d: %s", code, b)
	}
	ups := upsertRequest{Documents: []documentSpec{
		{ID: "a", Vectors: map[string][]float32{"e": {0.7}}},
		{ID: "b", Vectors: map[string][]float32{"e": {0.9}}},
	}}
	if code, b := do(t, ts, "POST", "/api/v1/collections/q1/documents", "admin", "admin", ups); code != http.StatusOK {
		t.Fatalf("upsert q1: got %d: %s", code, b)
	}
	code, b := do(t, ts, "POST", "/api/v1/collections/q1/query", "admin", "admin",
		queryRequest{Field: "e", Vector: []float32{1.0}, TopK: 2})
	if code != http.StatusOK {
		t.Fatalf("query 1-dim: got %d: %s", code, b)
	}
	var qr queryResponse
	decode(t, b, &qr)
	if len(qr.Results) != 2 {
		t.Errorf("1-dim query: got %d results, want 2", len(qr.Results))
	}
}

// ---- auth.go ----

// TestAuthenticatorUnit covers NewAuthenticator validation (empty name,
// duplicates, malformed sha256, empty list) and Authenticate semantics
// (plain vs hashed, readonly flag, unknown user) asserting both return
// values.
func TestAuthenticatorUnit(t *testing.T) {
	hexsum := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}

	a, err := NewAuthenticator([]User{
		{Username: "admin", Password: "sha256:" + hexsum("admin")},
		{Username: "viewer", Password: "sha256:" + hexsum("viewer"), Readonly: true},
		{Username: "plain", Password: "secret"},
	})
	if err != nil || a == nil {
		t.Fatalf("NewAuthenticator = %v, %v", a, err)
	}

	for _, tc := range []struct {
		user, pass string
		readonly   bool
		ok         bool
	}{
		{"admin", "admin", false, true},
		{"admin", "wrong", false, false},
		{"viewer", "viewer", true, true},
		{"viewer", "admin", false, false},
		{"plain", "secret", false, true},
		{"plain", "nope", false, false},
		{"ghost", "x", false, false},
	} {
		ro, ok := a.Authenticate(tc.user, tc.pass)
		if ro != tc.readonly || ok != tc.ok {
			t.Errorf("Authenticate(%q) = (%v, %v), want (%v, %v)", tc.user, ro, ok, tc.readonly, tc.ok)
		}
	}

	for name, users := range map[string][]User{
		"empty username":  {{Username: "   "}},
		"duplicate":       {{Username: "a"}, {Username: "a"}},
		"sha256 66 chars": {{Username: "a", Password: "sha256:" + strings.Repeat("ab", 33)}},
		"sha256 bad hex":  {{Username: "a", Password: "sha256:zzzz"}},
		"nil list":        nil,
		"empty list":      {},
	} {
		if a, err := NewAuthenticator(users); err == nil || a != nil {
			t.Errorf("NewAuthenticator(%s) = %v, %v; want (nil, err)", name, a, err)
		}
	}
}

// ---- server.go / web.go ----

// TestNewNilConfig kills the negated Init-error check in New (Init(nil)
// cannot fail, so the mutant would always error) and pins the default
// config/log the nil arguments produce.
func TestNewNilConfig(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	s, err := New(nil, nil)
	if err != nil {
		t.Fatalf("New(nil, nil) = %v, want nil error", err)
	}
	if s == nil || s.log == nil || s.cfg == nil {
		t.Fatalf("New returned incomplete server: %+v", s)
	}
	if s.cfg.Server.Addr != "127.0.0.1:8080" || s.cfg.Storage.DataDir != "./data" ||
		s.cfg.Server.ReadTimeoutSeconds != 30 {
		t.Errorf("default cfg not applied: %+v", s.cfg)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	code, b := do(t, ts, "GET", "/healthz", "", "", nil)
	if code != http.StatusOK {
		t.Errorf("health: got %d: %s", code, b)
	}
	s.mgr.Close()
}

// TestStartShutdownCycle exercises the real ListenAndServe path. A
// connection whose chunked request body never completes is held open so a
// short-timeout Shutdown cannot finish and must return the context error
// (kills the err != nil negation and nil-return mutants in Shutdown). Then
// a graceful Shutdown must make Start return nil (kills the errors.Is
// negation).
func TestStartShutdownCycle(t *testing.T) {
	// Get a free port via a throwaway listener, then let the real server
	// take it.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	cfg := &Config{
		Server:  ServerConfig{Addr: addr},
		Storage: StorageConfig{DataDir: t.TempDir()},
		Auth:    AuthConfig{Enabled: false},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()

	// Wait for the listener to accept connections.
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up: %v", err)
		}
		if conn != nil {
			conn.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Hold an active connection open: a chunked request whose body never
	// arrives blocks the handler inside decodeBody.
	reqLine := "POST /api/v1/collections HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Content-Type: application/json\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n"
	if _, err := conn.Write([]byte(reqLine)); err != nil {
		t.Fatalf("write request: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := s.Shutdown(ctx); err == nil {
		t.Error("Shutdown with an active stuck connection: err = nil, want context timeout")
	}
	conn.Close()

	// The server is still running; a graceful shutdown must now succeed and
	// make Start return nil.
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("graceful Shutdown: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start after graceful shutdown: %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
}

// TestNewFailsWhenDataDirIsBlocked kills the ReturnVals mutants on
// New's NewCollectionManager error return: a regular file squatting on the
// data dir makes MkdirAll fail, and the error (with a nil server) must be
// reported.
func TestNewFailsWhenDataDirIsBlocked(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldwd) })
	base := t.TempDir()
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("data", []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Server:  ServerConfig{Addr: "127.0.0.1:0"},
		Storage: StorageConfig{DataDir: "./data"},
		Auth:    AuthConfig{Enabled: false},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if srv, err := New(cfg, logger); err == nil || srv != nil {
		t.Errorf("New with blocked data dir = %v, %v; want (nil, err)", srv, err)
	}
}

// TestNewFailsWhenAuthEnabledWithoutUsers kills the nil-error return value
// in New's auth-configure error branch: an enabled auth config with no
// users cannot start.
func TestNewFailsWhenAuthEnabledWithoutUsers(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{Addr: "127.0.0.1:0"},
		Storage: StorageConfig{DataDir: t.TempDir()},
		Auth:    AuthConfig{Enabled: true},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if srv, err := New(cfg, logger); err == nil || srv != nil {
		t.Errorf("New(auth enabled, no users) = %v, %v; want (nil, err)", srv, err)
	}
}

// TestStartFailsWhenPortInUse kills the nil-error return value in Start:
// a second server on an already-bound address must surface the bind error
// instead of claiming success.
func TestStartFailsWhenPortInUse(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mk := func(dataDir string) *Server {
		cfg := &Config{
			Server:  ServerConfig{Addr: addr},
			Storage: StorageConfig{DataDir: dataDir},
			Auth:    AuthConfig{Enabled: false},
		}
		s, err := New(cfg, logger)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return s
	}

	first := mk(t.TempDir())
	errCh := make(chan error, 1)
	go func() { errCh <- first.Start() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("first server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	second := mk(t.TempDir())
	secondCh := make(chan error, 1)
	go func() { secondCh <- second.Start() }()
	select {
	case err := <-secondCh:
		if err == nil {
			t.Fatal("second Start on a busy port: err = nil, want bind error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second Start did not return (port conflict not detected)")
	}
	if err := first.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown: %v", err)
	}
	<-errCh
}

// TestHandleRootPaths pins the /api prefix boundary in handleRoot:
// "/api/" (length 5), "/api", and "/api/x" are JSON 404s, while "/" is the
// web UI.
func TestHandleRootPaths(t *testing.T) {
	ts := newTestServer(t, adminUsers())
	for _, p := range []string{"/api/", "/api", "/api/x"} {
		res, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: got %d, want 404", p, res.StatusCode)
		}
	}
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("GET /: got %d %q, want 200 text/html", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

// ---- helpers ----

// pad4 zero-pads to 4 digits (stable, lexicographically sortable ids).
func pad4(i int) string {
	return strconv.Itoa(i)
}

// numEqual compares a JSON-decoded number (float64) with want, tolerating
// the int64/int encodings the server may emit.
func numEqual(v interface{}, want float64) bool {
	switch x := v.(type) {
	case float64:
		return x == want
	case int64:
		return float64(x) == want
	case int:
		return float64(x) == want
	}
	return false
}
