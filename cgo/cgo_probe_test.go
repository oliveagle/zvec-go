//go:build cgo && zvec_cgo

package cgoz

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestVersionFromCoredb(t *testing.T) {
	v := VersionString()
	if v == "" {
		t.Fatal("VersionString() is empty; expected e.g. v0.7.0")
	}
	t.Logf("zvec core version: %s", v)
	if !CheckVersion(0, 7, 0) {
		t.Errorf("CheckVersion(0,7,0) = false, want true (got %s)", v)
	}
	if CheckVersion(9, 9, 9) {
		t.Errorf("CheckVersion(9,9,9) = true, want false")
	}
}

func TestCollectionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt_coll")

	schema, err := NewSchema("rt_coll")
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	defer schema.Free()
	if err := schema.AddInt64Field("ts", false); err != nil {
		t.Fatalf("AddInt64Field: %v", err)
	}
	if err := schema.AddStringField("title", true); err != nil {
		t.Fatalf("AddStringField: %v", err)
	}
	if err := schema.AddVectorField("emb", 4, IndexHNSW, MetricCosine, &HnswParams{M: 8, EfConstruction: 64}); err != nil {
		t.Fatalf("AddVectorField: %v", err)
	}

	coll, err := CreateAndOpen(path, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	defer coll.Close()

	docs := make([]*Document, 4)
	vecs := [][]float32{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0.9, 0.1, 0, 0},
	}
	for i := range docs {
		d, err := NewDocument(fmt.Sprintf("doc%d", i))
		if err != nil {
			t.Fatalf("NewDocument: %v", err)
		}
		if err := d.SetInt64("ts", int64(i)); err != nil {
			t.Fatalf("SetInt64: %v", err)
		}
		if err := d.SetString("title", fmt.Sprintf("t%d", i)); err != nil {
			t.Fatalf("SetString: %v", err)
		}
		if err := d.SetVectorFP32("emb", vecs[i]); err != nil {
			t.Fatalf("SetVectorFP32: %v", err)
		}
		docs[i] = d
	}
	defer func() {
		for _, d := range docs {
			d.Free()
		}
	}()

	written, failed, err := coll.Upsert(docs...)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 4 || failed != 0 {
		t.Fatalf("Upsert written=%d failed=%d, want 4/0", written, failed)
	}

	q, err := NewQuery("emb", vecs[0], 3)
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	defer q.Free()
	results, err := coll.Query(q)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Free()
		}
	}()
	if len(results) == 0 {
		t.Fatal("Query returned 0 results, want > 0")
	}
	// The exact-match vector (doc0) must rank first.
	if got := results[0].PK(); got != "doc0" {
		t.Errorf("top result = %q, want doc0 (scores: %v)", got, scoresOf(results))
	}
	// Similar vector (doc3) should be within the top 2.
	if got := results[1].PK(); got != "doc3" {
		t.Errorf("2nd result = %q, want doc3", got)
	}

	// Read back a scalar field from the top result.
	if ts, err := results[0].GetInt64("ts"); err != nil || ts != 0 {
		t.Errorf("GetInt64(ts) on top result = %d, %v; want 0", ts, err)
	}

	// Delete one doc and confirm it no longer ranks.
	if _, failed, err := coll.Delete("doc0"); err != nil || failed != 0 {
		t.Fatalf("Delete(doc0): failed=%d err=%v", failed, err)
	}
	results, err = coll.Query(q)
	if err != nil {
		t.Fatalf("Query after delete: %v", err)
	}
	for _, r := range results {
		if r.PK() == "doc0" {
			t.Error("deleted doc0 still returned by query")
		}
		r.Free()
	}

	// Persistence: close, reopen, query still works.
	coll.Close()
	coll2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open after close: %v", err)
	}
	defer coll2.Close()
	results2, err := coll2.Query(q)
	if err != nil {
		t.Fatalf("Query after reopen: %v", err)
	}
	for _, r := range results2 {
		r.Free()
	}
	if len(results2) == 0 {
		t.Fatal("no results after reopen; persistence broken")
	}

	// Destroy removes the data.
	if err := coll2.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

func scoresOf(docs []*Document) []float64 {
	out := make([]float64, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.Score())
	}
	return out
}

func TestSchemaNameValidation(t *testing.T) {
	for _, bad := range []string{"", "rt", "ab c", "has/slash"} {
		if _, err := NewSchema(bad); err == nil {
			t.Errorf("NewSchema(%q) = nil error, want validation error", bad)
		}
	}
	long := "a" + "b" + "c"
	for i := 0; i < 64; i++ {
		long += "a"
	}
	if _, err := NewSchema(long); err == nil {
		t.Errorf("NewSchema(67-char name) = nil error, want validation error")
	}
	s, err := NewSchema("ok_name-1")
	if err != nil {
		t.Fatalf("NewSchema(ok_name-1): %v", err)
	}
	defer s.Free()
	if err := s.AddStringField("bad name", true); err == nil {
		t.Error("AddStringField(bad name) = nil error, want validation error")
	}
	if err := s.AddInt64Field("ok_field", false); err != nil {
		t.Errorf("AddInt64Field(ok_field): %v", err)
	}
}
