package zvec

// Focused tests that close the test gaps revealed by mutation testing
// (gomut). Each test targets a specific mutant class: boundary conditions,
// error propagation, and branch coverage in the client library.
//
// Surviving mutants intentionally not chased here are equivalent or
// unobservable:
//   - Search/Query: sort comparator ">" vs ">=" (stable sort keeps equal
//     scores in input order) and "len(results) > topK" vs ">=" (identical
//     truncation when topK == count).
//   - ListDocs: "offset < 0" vs "<=", "offset > len" vs ">=" (identical
//     clamping), "limit > 0" vs ">=" when limit equals the remaining count.
//   - ValidateDocID: the "./../..\.." segment checks are unreachable (any
//     id containing them was already rejected by the separator check), so
//     the BooleanSwap and error-return mutants there are equivalent.
//   - NewQueryExecutor: Execute never reads the stored schema, so the
//     zero-value/nil executor behaves identically.
//   - writeFileAtomic MkdirAll-failure return: the following CreateTemp
//     fails for the same reason, so the observable error is the same.
//   - loadDocs: the nil-map guard (the constructor always allocates the
//     map) and the ReadDir-entry ReadFile branch (directories are skipped
//     before it) are unreachable in practice.
//   - math.Inf(-1) constant mutants: any negative argument still yields
//     -Inf, so value-preserving rewrites are equivalent.
//   - ListDocs "offset < 0" written as "offset < 1": both clamp offset=0
//     to the same value.
//   - metricFor's final default return: an empty metric type and the
//     COSINE default take the same switch branch in similarity().
//   - readDocument's ValidateDocID re-check: dead (callers already
//     validate), so its error return is unreachable with a distinguishing
//     input.
//   - Schema constructors whose defaults are all zero values
//     (NewInvertIndexParam, Default*Option): the zero-value replacement is
//     indistinguishable.
//   - Config.ToJSON error return: Config only holds JSON-safe fields, so
//     marshalling cannot fail (dead branch).
//   - File-mode constants (0755/0644) in MkdirAll/WriteFile: modes do not
//     affect behavior here.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newMutationColl(t *testing.T) *Collection {
	t.Helper()
	return newMutationCollSchema(t, func() *CollectionSchema {
		schema := NewCollectionSchema("mut")
		schema.AddField(NewFieldSchema("n", DataTypeInt64))
		schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 4))
		return schema
	})
}

func newMutationCollSchema(t *testing.T, mk func() *CollectionSchema) *Collection {
	t.Helper()
	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	schema := mk()
	coll, err := CreateAndOpen(filepath.Join(t.TempDir(), "mut"), schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	t.Cleanup(func() { coll.Close() })
	return coll
}

// TestNewDocumentInitialised kills ReturnVals mutants on NewDocument.
func TestNewDocumentInitialised(t *testing.T) {
	d := NewDocument("nd1")
	if d == nil || d.ID != "nd1" {
		t.Fatalf("NewDocument = %+v, want non-nil doc with ID nd1", d)
	}
	if d.Fields == nil || d.Vectors == nil || d.Metadata == nil {
		t.Fatal("NewDocument maps not initialised")
	}
	d.SetField("k", "v").SetVector("e", []float32{1}).SetMetadata("m", 1)
	if got, ok := d.GetField("k"); !ok || got != "v" {
		t.Errorf("GetField = %v, %v; want v", got, ok)
	}
	if v, ok := d.GetVector("e"); !ok || len(v) != 1 {
		t.Errorf("GetVector = %v, %v; want 1-elem", v, ok)
	}
}

// TestInsertErrorBranches kills the nil/empty-id/validation mutants in
// Insert and InsertBatch (including the count bookkeeping).
func TestInsertErrorBranches(t *testing.T) {
	coll := newMutationColl(t)
	if err := coll.Insert(nil); err == nil {
		t.Error("Insert(nil): want error, got nil")
	}
	if err := coll.Insert(&Document{ID: ""}); err == nil {
		t.Error("Insert(empty id): want error, got nil")
	}
	if err := coll.Insert(NewDocument("a/b")); err == nil {
		t.Error("Insert(path id): want error, got nil")
	}
	// A NUL-only malformed id (no separators) must still be rejected and
	// must not be added to the in-memory map.
	if err := coll.Insert(NewDocument("x\x00y")); err == nil {
		t.Error("Insert(NUL id): want error, got nil")
	}
	if n, _ := coll.Count(); n != 0 {
		t.Errorf("Count after rejected inserts = %d, want 0", n)
	}

	coll2 := newMutationColl(t)
	n, err := coll2.InsertBatch([]*Document{
		NewDocument("i1").SetField("n", int64(1)),
		NewDocument("i2"),
	})
	if err != nil || n != 2 {
		t.Fatalf("InsertBatch(2 valid) = %d, %v; want 2, nil", n, err)
	}
	n, err = coll2.InsertBatch([]*Document{NewDocument("i3"), nil})
	if err == nil || n != 1 {
		t.Errorf("InsertBatch(valid, nil) = %d, %v; want 1, err", n, err)
	}
	n, err = coll2.InsertBatch([]*Document{NewDocument("i4"), NewDocument("i5/i6")})
	if err == nil || n != 1 {
		t.Errorf("InsertBatch(valid, bad-id) = %d, %v; want 1, err", n, err)
	}
}

// TestUpdateErrorBranches kills the nil/empty-id/validation mutants in
// Update, and proves a successful update is visible to Get.
func TestUpdateErrorBranches(t *testing.T) {
	coll := newMutationColl(t)
	if err := coll.Update(nil); err == nil {
		t.Error("Update(nil): want error, got nil")
	}
	if err := coll.Update(&Document{ID: ""}); err == nil {
		t.Error("Update(empty id): want error, got nil")
	}
	if err := coll.Update(NewDocument("a/b")); err == nil {
		t.Error("Update(path id): want error, got nil")
	}

	d := NewDocument("u1")
	d.SetField("n", int64(1))
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	d.SetField("n", int64(2))
	if err := coll.Update(d); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := coll.Get("u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v, _ := got.Fields["n"]; v != int64(2) {
		t.Errorf("after Update, Get = %v, want 2", v)
	}
}

// TestDeleteErrorBranches kills the Delete validation / missing-doc
// mutants.
func TestDeleteErrorBranches(t *testing.T) {
	coll := newMutationColl(t)
	if err := coll.Insert(NewDocument("del1")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Missing document: stat fails, must wrap ErrDocNotFound.
	if err := coll.Delete("ghost"); !errors.Is(err, ErrDocNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrDocNotFound", err)
	}
	// Invalid id must be rejected before touching the disk.
	if err := coll.Delete("a/b"); err == nil {
		t.Error("Delete(path id): want error, got nil")
	}
	// A deleted document disappears from memory and disk.
	if err := coll.Delete("del1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := coll.Get("del1"); !errors.Is(err, ErrDocNotFound) {
		t.Errorf("Get(deleted) = %v, want ErrDocNotFound", err)
	}
}

// TestClosedCollectionErrors kills the closed-flag mutants in every
// public collection method.
func TestClosedCollectionErrors(t *testing.T) {
	coll := newMutationColl(t)
	d := NewDocument("c1")
	d.SetField("n", int64(1))
	d.SetVector("emb", []float32{1, 0, 0, 0})
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	coll.Close()

	q := NewVectorQueryByVector("emb", []float32{1, 0, 0, 0})
	// Each fn returns an error only when the call MISbehaved (returned
	// no error, or an unexpected value).
	checks := []struct {
		name string
		fn   func() error
	}{
		{"Insert", func() error {
			if err := coll.Insert(d); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"InsertBatch", func() error {
			n, err := coll.InsertBatch([]*Document{d})
			if err == nil || n != 0 {
				return fmt.Errorf("got n=%d err=%v", n, err)
			}
			return nil
		}},
		{"Update", func() error {
			if err := coll.Update(d); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Upsert", func() error {
			if err := coll.Upsert(d); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Get", func() error {
			got, err := coll.Get("c1")
			if err == nil || got != nil {
				return fmt.Errorf("got doc=%v err=%v", got, err)
			}
			return nil
		}},
		{"Delete", func() error {
			if err := coll.Delete("c1"); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Fetch", func() error {
			if _, err := coll.Fetch([]string{"c1"}); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Search", func() error {
			if _, err := coll.Search(q); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Query", func() error {
			if _, err := coll.Query(q, 1, "", false, nil); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Count", func() error {
			n, err := coll.Count()
			if err == nil || n != 0 {
				return fmt.Errorf("got n=%d err=%v", n, err)
			}
			return nil
		}},
		{"ListIDs", func() error {
			if _, err := coll.ListIDs(); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"ListDocs", func() error {
			if _, err := coll.ListDocs(0, 0); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Stats", func() error {
			st, err := coll.Stats()
			if err == nil || st != nil {
				return fmt.Errorf("got st=%v err=%v", st, err)
			}
			return nil
		}},
		{"CreateIndex", func() error {
			if err := coll.CreateIndex("emb", NewHnswIndexParam(), DefaultIndexOption()); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"DropIndex", func() error {
			if err := coll.DropIndex("emb"); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Optimize", func() error {
			if err := coll.Optimize(DefaultOptimizeOption()); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"Flush", func() error {
			if err := coll.Flush(); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"AddColumn", func() error {
			if err := coll.AddColumn(NewFieldSchema("c", DataTypeInt64), "", nil); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"DropColumn", func() error {
			if err := coll.DropColumn("n"); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
		{"AlterColumn", func() error {
			if err := coll.AlterColumn("n", "c2", nil, nil); err == nil {
				return errors.New("no error")
			}
			return nil
		}},
	}
	for _, c := range checks {
		if err := c.fn(); err != nil {
			t.Errorf("%s after Close: %v", c.name, err)
		}
	}
}

// TestValidateDocIDBackslashTraversal kills the &&-mutants of the
// traversal checks: a\..\b is not a traversal on Linux (Base == id) so
// only the explicit backslash-dotdot check rejects it.
func TestValidateDocIDBackslashTraversal(t *testing.T) {
	for _, id := range []string{`a\..\b`, `\..\x`, `x\..\..\y`} {
		if err := ValidateDocID(id); err == nil {
			t.Errorf("ValidateDocID(%q) = nil, want traversal error", id)
		}
	}
	// NUL without any separator must be rejected by the separator check.
	if err := ValidateDocID("x\x00y"); err == nil {
		t.Error("ValidateDocID(NUL id) = nil, want error")
	}
	// Bare dot ids are rejected.
	if err := ValidateDocID("."); err == nil {
		t.Error("ValidateDocID(.) = nil, want error")
	}
	if err := ValidateDocID(".."); err == nil {
		t.Error("ValidateDocID(..) = nil, want error")
	}
}

// TestDocumentSetterChains kills the ReturnVals mutants on the
// Document setter chain methods (each must return its receiver).
func TestDocumentSetterChains(t *testing.T) {
	d := NewDocument("ch")
	if d.SetField("f", 1) != d {
		t.Error("SetField must return the receiver")
	}
	if d.SetVector("v", []float32{1, 2}) != d {
		t.Error("SetVector must return the receiver")
	}
	if d.SetMetadata("m", "x") != d {
		t.Error("SetMetadata must return the receiver")
	}
}

// TestReadDocumentDirect kills the readDocument error-return mutants
// (validation, missing file, corrupt file) by calling the internal
// reader directly, where Get's disk fallback would mask the results.
func TestReadDocumentDirect(t *testing.T) {
	coll := newMutationColl(t)
	d := NewDocument("rd1")
	d.SetField("n", int64(4))
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if doc, err := coll.readDocument("a/b"); err == nil || doc != nil {
		t.Errorf("readDocument(bad id) = %v, %v; want nil doc, err", doc, err)
	}
	doc, err := coll.readDocument("missing")
	if err == nil || doc != nil {
		t.Fatalf("readDocument(missing) = %v, %v; want nil doc, err", doc, err)
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("readDocument(missing) = %q; want 'no such file'", err)
	}
	os.WriteFile(coll.docPath("rdbad"), []byte("{bad"), 0644)
	if doc, err := coll.readDocument("rdbad"); err == nil || doc != nil {
		t.Errorf("readDocument(corrupt) = %v, %v; want nil doc, err", doc, err)
	}
	doc, err = coll.readDocument("rd1")
	if err != nil || doc == nil {
		t.Fatalf("readDocument(good) = %v, %v; want doc, nil", doc, err)
	}
	if v, _ := doc.Fields["n"]; v != int64(4) && v != float64(4) {
		t.Errorf("rd1.n = %v, want 4", v)
	}
}

// TestPersistSchemaMarshalFailure kills the persistSchema error-return
// mutant by making the metadata unmarshalable (chan as index param).
func TestPersistSchemaMarshalFailure(t *testing.T) {
	schema := NewCollectionSchema("ps")
	schema.AddField(NewFieldSchema("n", DataTypeInt64))
	vs := NewVectorSchema("v", DataTypeVectorFP32, 2)
	vs.IndexParam = make(chan int) // not JSON-marshalable
	schema.AddVectorField(vs)
	coll := &Collection{
		path:   t.TempDir(),
		schema: schema,
		option: DefaultCollectionOption(),
		docs:   make(map[string]*Document),
	}
	if err := coll.persistSchema(); err == nil {
		t.Error("persistSchema(unmarshalable schema): want marshal error, got nil")
	}
}

// TestDeleteByFilterPlaceholders pins the placeholder messages so the
// closed/open branches stay distinguishable.
func TestDeleteByFilterPlaceholders(t *testing.T) {
	coll := newMutationColl(t)
	if err := coll.DeleteByFilter("n > 1"); err == nil || !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("DeleteByFilter(open) = %v; want 'not yet implemented'", err)
	}
	coll.Close()
	if err := coll.DeleteByFilter("n > 1"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("DeleteByFilter(closed) = %v; want 'closed'", err)
	}
}

// TestGetDiskFallbackAndErrors kills the Get in-memory-branch,
// disk-fallback, and not-found error-propagation mutants.
func TestGetDiskFallbackAndErrors(t *testing.T) {
	coll := newMutationColl(t)

	// In-memory value differs from disk: update in place after persistence.
	d := NewDocument("g1")
	d.SetField("n", int64(1))
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	d.SetField("n", int64(2)) // mutate the shared in-memory doc only

	got, err := coll.Get("g1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get(existing): nil doc")
	}
	if v, _ := got.Fields["n"]; v != int64(2) {
		t.Errorf("Get returned stale (disk) value %v, want in-memory 2", v)
	}

	// ID absent from map and disk must produce ErrDocNotFound with a
	// nil doc, and the underlying filesystem error must be the read
	// failure (not a JSON parse failure), which distinguishes the
	// ReadFile-branch mutants.
	_, err = coll.Get("never-there")
	if err == nil {
		t.Fatal("Get(missing): want error")
	}
	if !errors.Is(err, ErrDocNotFound) {
		t.Fatalf("Get(missing) = %v, want ErrDocNotFound", err)
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("Get(missing) = %q, want underlying 'no such file' error", err)
	}

	// Invalid id must be rejected with a nil doc.
	got, err = coll.Get("a/b")
	if err == nil || got != nil {
		t.Errorf("Get(bad id) = %v, %v; want nil doc, err", got, err)
	}

	// Corrupt file: map cleared, garbage on disk -> error, nil doc.
	coll.mu.Lock()
	coll.docs = make(map[string]*Document)
	coll.mu.Unlock()
	os.WriteFile(coll.docPath("corrupt"), []byte("{not json"), 0644)
	got, err = coll.Get("corrupt")
	if err == nil || got != nil {
		t.Errorf("Get(corrupt file) = %v, %v; want nil doc, err", got, err)
	}

	// Disk fallback: id absent from the (cleared) map but present on
	// disk must be loaded and returned.
	got, err = coll.Get("g1")
	if err != nil || got == nil {
		t.Fatalf("Get(disk-only) = %v, %v; want the disk copy", got, err)
	}
	if v, _ := got.Fields["n"]; v != int64(1) && v != float64(1) {
		t.Errorf("Get(disk-only) n = %v, want 1 (persisted value)", v)
	}
}

// TestCountAndListIDs kills the Count/ListIDs return-value and constant
// mutants.
func TestCountAndListIDs(t *testing.T) {
	coll := newMutationColl(t)
	if n, err := coll.Count(); err != nil || n != 0 {
		t.Fatalf("Count(empty) = %d, %v; want 0, nil", n, err)
	}
	if ids, err := coll.ListIDs(); err != nil || len(ids) != 0 {
		t.Fatalf("ListIDs(empty) = %v, %v; want none", ids, err)
	}

	for _, id := range []string{"b", "a", "c"} {
		if err := coll.Insert(NewDocument(id)); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	if n, _ := coll.Count(); n != 3 {
		t.Errorf("Count = %d, want 3", n)
	}
	ids, err := coll.ListIDs()
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(ids) != 3 {
		t.Fatalf("ListIDs = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ListIDs = %v, want sorted %v", ids, want)
		}
	}
}

// TestSearchTopkBoundaries kills TopK boundary/constant mutants in Search.
func TestSearchTopkBoundaries(t *testing.T) {
	coll := newMutationColl(t)
	for i := 0; i < 3; i++ {
		d := NewDocument("s" + string(rune('a'+i)))
		d.SetVector("emb", []float32{float32(i) * 0.1, 1, 0, 0})
		if err := coll.Insert(d); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	// TopK = 0 means "no limit": all 3 results.
	res, err := coll.Search(NewVectorQueryByVector("emb", []float32{0, 1, 0, 0}).WithTopK(0))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 3 {
		t.Errorf("Search TopK=0: got %d results, want 3 (no limit)", len(res))
	}
	// TopK exactly equal to the result count: nothing truncated.
	res, _ = coll.Search(NewVectorQueryByVector("emb", []float32{0, 1, 0, 0}).WithTopK(3))
	if len(res) != 3 {
		t.Errorf("Search TopK=3 (== count): got %d results, want 3", len(res))
	}
	// TopK=1: exactly one result.
	res, _ = coll.Search(NewVectorQueryByVector("emb", []float32{0, 1, 0, 0}).WithTopK(1))
	if len(res) != 1 {
		t.Errorf("Search TopK=1: got %d results, want 1", len(res))
	}
	// String() must render id and score (kills its constant-return mutant).
	if s := res[0].String(); !strings.Contains(s, res[0].ID) || !strings.Contains(s, "Score") {
		t.Errorf("SearchResult.String() = %q; want id and score", s)
	}
}

// TestSearchByID kills the Search-by-document-id branch mutants
// (missing id, id without the requested vector field).
func TestSearchByID(t *testing.T) {
	coll := newMutationColl(t)
	d := NewDocument("sby")
	d.SetVector("emb", []float32{1, 0, 0, 0})
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	d2 := NewDocument("nov")
	d2.SetField("n", int64(1))
	if err := coll.Insert(d2); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	res, err := coll.Search(NewVectorQueryByID("emb", "sby"))
	if err != nil {
		t.Fatalf("Search(by existing id): %v", err)
	}
	if len(res) != 1 || res[0].ID != "sby" {
		t.Errorf("Search(by id) = %d results, want 1 for sby", len(res))
	}
	if _, err := coll.Search(NewVectorQueryByID("emb", "ghost")); !errors.Is(err, ErrDocNotFound) {
		t.Errorf("Search(by missing id) = %v, want ErrDocNotFound", err)
	}
	if _, err := coll.Search(NewVectorQueryByID("emb", "nov")); err == nil {
		t.Error("Search(by id without vector): want error, got nil")
	}
}

// TestSearchInvalidQueries kills the Search query-validation error
// return mutants.
func TestSearchInvalidQueries(t *testing.T) {
	coll := newMutationColl(t)
	if _, err := coll.Search(NewVectorQueryByVector("", []float32{1, 0, 0, 0})); err == nil {
		t.Error("Search(empty field name): want error, got nil")
	}
	if _, err := coll.Search(&VectorQuery{FieldName: "emb"}); err == nil {
		t.Error("Search(neither id nor vector): want error, got nil")
	}
	if _, err := coll.Search(&VectorQuery{FieldName: "emb", ID: "x", Vector: []float32{1}}); err == nil {
		t.Error("Search(both id and vector): want error, got nil")
	}
}

// TestQueryTopkBoundaries kills the analogous mutants in Query.
func TestQueryTopkBoundaries(t *testing.T) {
	coll := newMutationColl(t)
	for i := 0; i < 3; i++ {
		d := NewDocument("q" + string(rune('a'+i)))
		d.SetVector("emb", []float32{float32(i) * 0.1, 1, 0, 0})
		if err := coll.Insert(d); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	q := NewVectorQueryByVector("emb", []float32{0, 1, 0, 0})
	if res, err := coll.Query(q, 0, "", false, nil); err != nil || len(res) != 3 {
		t.Errorf("Query topk=0: n=%d err=%v; want 3", len(res), err)
	}
	if res, err := coll.Query(q, 3, "", false, nil); err != nil || len(res) != 3 {
		t.Errorf("Query topk==count: n=%d err=%v; want 3", len(res), err)
	}
	if res, err := coll.Query(q, 1, "", false, nil); err != nil || len(res) != 1 {
		t.Errorf("Query topk=1: n=%d err=%v; want 1", len(res), err)
	}
}

// TestQueryFieldSelection kills the outputFields/includeVector branch
// mutants in Query.
func TestQueryFieldSelection(t *testing.T) {
	coll := newMutationColl(t)
	d := NewDocument("qf1")
	d.SetField("n", int64(7))
	d.SetField("x", "s")
	d.SetVector("emb", []float32{1, 0, 0, 0})
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	q := NewVectorQueryByVector("emb", []float32{1, 0, 0, 0})

	// outputFields=nil: all fields, no vectors.
	res, err := coll.Query(q, 10, "", false, nil)
	if err != nil || len(res) != 1 {
		t.Fatalf("Query: %v, %d results", err, len(res))
	}
	if _, ok := res[0].Fields["n"]; !ok {
		t.Error("Query(all fields): missing n")
	}
	if _, ok := res[0].Fields["x"]; !ok {
		t.Error("Query(all fields): missing x")
	}
	if len(res[0].Vector) != 0 {
		t.Errorf("Query(includeVector=false): %d vectors returned, want 0", len(res[0].Vector))
	}

	// outputFields=["n"]: only that field.
	res, err = coll.Query(q, 10, "", false, []string{"n"})
	if err != nil || len(res) != 1 {
		t.Fatalf("Query(selected): %v, %d results", err, len(res))
	}
	if v, ok := res[0].Fields["n"]; !ok || v != int64(7) {
		t.Errorf("Query(selected): Fields[n] = %v, %v; want 7", v, ok)
	}
	if _, ok := res[0].Fields["x"]; ok {
		t.Error("Query(selected): x must not be included")
	}
	if len(res[0].Fields) != 1 {
		t.Errorf("Query(selected): %d fields returned, want 1", len(res[0].Fields))
	}

	// includeVector=true: vector map populated.
	res, err = coll.Query(q, 10, "", true, nil)
	if err != nil || len(res) != 1 {
		t.Fatalf("Query(includeVector): %v, %d results", err, len(res))
	}
	if vec, ok := res[0].Vector["emb"]; !ok || len(vec) != 4 {
		t.Errorf("Query(includeVector=true): emb vector = %v, %v; want 4-elem", vec, ok)
	}
}

// TestQueryByID kills the Query-by-document-id branch mutants
// (missing id, id without the requested vector field).
func TestQueryByID(t *testing.T) {
	coll := newMutationColl(t)
	d := NewDocument("qby")
	d.SetField("n", int64(5))
	d.SetVector("emb", []float32{1, 0, 0, 0})
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	d2 := NewDocument("qnov")
	d2.SetField("n", int64(1))
	if err := coll.Insert(d2); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	res, err := coll.Query(NewVectorQueryByID("emb", "qby"), 1, "", false, nil)
	if err != nil {
		t.Fatalf("Query(by existing id): %v", err)
	}
	if len(res) != 1 || res[0].ID != "qby" {
		t.Errorf("Query(by id) = %d results, want 1 for qby", len(res))
	}
	if _, err := coll.Query(NewVectorQueryByID("emb", "ghost"), 1, "", false, nil); !errors.Is(err, ErrDocNotFound) {
		t.Errorf("Query(by missing id) = %v, want ErrDocNotFound", err)
	}
	if _, err := coll.Query(NewVectorQueryByID("emb", "qnov"), 1, "", false, nil); err == nil {
		t.Error("Query(by id without vector): want error, got nil")
	}
}

// TestQueryInvalidQueries kills the Query query-validation error return
// mutants.
func TestQueryInvalidQueries(t *testing.T) {
	coll := newMutationColl(t)
	if _, err := coll.Query(NewVectorQueryByVector("", []float32{1}), 1, "", false, nil); err == nil {
		t.Error("Query(empty field name): want error, got nil")
	}
	if _, err := coll.Query(&VectorQuery{FieldName: "emb"}, 1, "", false, nil); err == nil {
		t.Error("Query(neither id nor vector): want error, got nil")
	}
}

// TestListDocsBoundaries kills the offset/limit boundary and constant
// mutants.
func TestListDocsBoundaries(t *testing.T) {
	coll := newMutationColl(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := coll.Insert(NewDocument(id).SetField("n", int64(1))); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	cases := []struct {
		offset, limit, want int
	}{
		{-5, 0, 3}, // negative offset clamps to 0; limit 0 = no limit
		{0, 0, 3},  // no offset, no limit
		{3, 0, 0},  // offset == len: empty
		{99, 0, 0}, // offset > len clamps to len: empty
		{0, 3, 3},  // limit == len: nothing truncated
		{0, 10, 3}, // limit > len
		{1, 2, 2},  // middle slice
		{2, 1, 1},
		{1, 1, 1}, // limit == 1 must truncate a longer remainder
	}
	for _, tc := range cases {
		got, err := coll.ListDocs(tc.offset, tc.limit)
		if err != nil {
			t.Fatalf("ListDocs(%d,%d): %v", tc.offset, tc.limit, err)
		}
		if len(got) != tc.want {
			t.Errorf("ListDocs(offset=%d, limit=%d) = %d docs, want %d", tc.offset, tc.limit, len(got), tc.want)
		}
	}
	// Empty collection: zero docs, no error.
	empty := newMutationColl(t)
	if got, err := empty.ListDocs(0, 0); err != nil || len(got) != 0 {
		t.Errorf("ListDocs on empty = %d docs, err %v; want 0", len(got), err)
	}
}

// TestMetricTypesChangeRanking kills metricFor mutants: each metric type
// must take a distinct code path and produce a distinct winner.
//
//	query q = [1, 0]
//	far   A = [2, 0]      (IP 2.0, COSINE 1.0, L2 d2=1)
//	near  B = [1.5, 0.5]  (IP 1.5, COSINE 0.9487, L2 d2=0.5)
//
//	IP     -> A (2.0 > 1.5)
//	COSINE -> A (1.0 > 0.9487)
//	L2     -> B (-0.5 > -1)
func TestMetricTypesChangeRanking(t *testing.T) {
	run := func(metric MetricType) []string {
		globalZvec = nil
		once = sync.Once{}
		if err := Init(DefaultConfig()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		schema := NewCollectionSchema("m" + string(metric))
		schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 2).WithMetricType(metric))
		coll, err := CreateAndOpen(filepath.Join(t.TempDir(), "m"+string(metric)), schema, nil)
		if err != nil {
			t.Fatalf("CreateAndOpen: %v", err)
		}
		defer coll.Close()
		dA := NewDocument("far")
		dA.SetVector("emb", []float32{2, 0})
		dB := NewDocument("near")
		dB.SetVector("emb", []float32{1.5, 0.5})
		if err := coll.Insert(dA); err != nil || coll.Insert(dB) != nil {
			t.Fatalf("Insert: %v", err)
		}
		res, err := coll.Search(NewVectorQueryByVector("emb", []float32{1, 0}).WithTopK(2))
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		order := make([]string, 0, len(res))
		for _, r := range res {
			order = append(order, r.ID)
		}
		return order
	}

	if o := run(MetricTypeL2); len(o) != 2 || o[0] != "near" {
		t.Errorf("L2 order = %v, want near first (smaller distance)", o)
	}
	if o := run(MetricTypeCOSINE); len(o) != 2 || o[0] != "far" {
		t.Errorf("COSINE order = %v, want far first (higher similarity)", o)
	}
	if o := run(MetricTypeIP); len(o) != 2 || o[0] != "far" {
		t.Errorf("IP order = %v, want far first (higher dot product)", o)
	}
}

// TestMetricForSchemaMutants kills the metricFor schema/field-matching
// mutants (schema==nil swap, && vs ||, != vs ==) using a decoy vector
// field and an explicit-metric target field, plus the default-return
// mutant via a field with an empty metric type.
func TestMetricForSchemaMutants(t *testing.T) {
	mk := func(fields []*VectorSchema) *Collection {
		globalZvec = nil
		once = sync.Once{}
		if err := Init(DefaultConfig()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		schema := NewCollectionSchema("mf")
		for _, f := range fields {
			schema.AddVectorField(f)
		}
		coll, err := CreateAndOpen(filepath.Join(t.TempDir(), "mf"), schema, nil)
		if err != nil {
			t.Fatalf("CreateAndOpen: %v", err)
		}
		t.Cleanup(func() { coll.Close() })
		return coll
	}
	insert := func(coll *Collection) {
		dA := NewDocument("far")
		dA.SetVector("emb", []float32{2, 0})
		dB := NewDocument("near")
		dB.SetVector("emb", []float32{1.5, 0.5})
		if err := coll.Insert(dA); err != nil || coll.Insert(dB) != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	first := func(coll *Collection) string {
		res, err := coll.Search(NewVectorQueryByVector("emb", []float32{1, 0}).WithTopK(1))
		if err != nil || len(res) != 1 {
			t.Fatalf("Search: %v, %d results", err, len(res))
		}
		return res[0].ID
	}

	// (a) A decoy field "o" (L2) precedes the target "emb" (IP). Any
	// mutant that makes metricFor pick the decoy or the COSINE default
	// flips the winner from "far" to "near".
	coll := mk([]*VectorSchema{
		NewVectorSchema("o", DataTypeVectorFP32, 2).WithMetricType(MetricTypeL2),
		NewVectorSchema("emb", DataTypeVectorFP32, 2).WithMetricType(MetricTypeIP),
	})
	insert(coll)
	if got := first(coll); got != "far" {
		t.Errorf("explicit IP metric: first = %q, want far", got)
	}

	// (b) Empty metric type falls through to the COSINE default.
	coll = mk([]*VectorSchema{
		{Name: "emb", DataType: DataTypeVectorFP32, Dimension: 2, MetricType: ""},
	})
	insert(coll)
	if got := first(coll); got != "far" {
		t.Errorf("default metric: first = %q, want far (cosine)", got)
	}
}

// TestExactMetricScores kills the math mutants in similarity /
// cosineSimilarity (operator swaps and constants) by asserting exact
// scores, zero-vector handling, and length-mismatch handling.
func TestExactMetricScores(t *testing.T) {
	mk := func(metric MetricType) *Collection {
		globalZvec = nil
		once = sync.Once{}
		if err := Init(DefaultConfig()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		schema := NewCollectionSchema("es")
		schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 2).WithMetricType(metric))
		coll, err := CreateAndOpen(filepath.Join(t.TempDir(), "es"), schema, nil)
		if err != nil {
			t.Fatalf("CreateAndOpen: %v", err)
		}
		t.Cleanup(func() { coll.Close() })
		return coll
	}
	scoreOf := func(coll *Collection, id string) (float64, error) {
		res, err := coll.Search(NewVectorQueryByVector("emb", []float32{1, 0}).WithTopK(5))
		if err != nil {
			return 0, err
		}
		for _, r := range res {
			if r.ID == id {
				return r.Score, nil
			}
		}
		return math.Inf(-1), errors.New("not in results")
	}
	closeTo := func(got, want float64) bool {
		return math.Abs(got-want) < 1e-9
	}

	collIP := mk(MetricTypeIP)
	dA := NewDocument("far")
	dA.SetVector("emb", []float32{2, 0})
	dB := NewDocument("near")
	dB.SetVector("emb", []float32{1.5, 0.5})
	if err := collIP.Insert(dA); err != nil || collIP.Insert(dB) != nil {
		t.Fatalf("Insert: %v", err)
	}
	if s, err := scoreOf(collIP, "far"); err != nil || !closeTo(s, 2.0) {
		t.Errorf("IP score(far) = %v, %v; want 2.0", s, err)
	}
	if s, err := scoreOf(collIP, "near"); err != nil || !closeTo(s, 1.5) {
		t.Errorf("IP score(near) = %v, %v; want 1.5", s, err)
	}

	collL2 := mk(MetricTypeL2)
	dA2 := NewDocument("far")
	dA2.SetVector("emb", []float32{2, 0})
	dB2 := NewDocument("near")
	dB2.SetVector("emb", []float32{1.5, 0.5})
	if err := collL2.Insert(dA2); err != nil || collL2.Insert(dB2) != nil {
		t.Fatalf("Insert: %v", err)
	}
	if s, err := scoreOf(collL2, "far"); err != nil || !closeTo(s, -1.0) {
		t.Errorf("L2 score(far) = %v, %v; want -1.0", s, err)
	}
	if s, err := scoreOf(collL2, "near"); err != nil || !closeTo(s, -0.5) {
		t.Errorf("L2 score(near) = %v, %v; want -0.5", s, err)
	}

	collCos := mk(MetricTypeCOSINE)
	dA3 := NewDocument("far")
	dA3.SetVector("emb", []float32{2, 0})
	dB3 := NewDocument("near")
	dB3.SetVector("emb", []float32{1.5, 0.5})
	dZ := NewDocument("zero")
	dZ.SetVector("emb", []float32{0, 0})
	if err := collCos.Insert(dA3); err != nil || collCos.Insert(dB3) != nil || collCos.Insert(dZ) != nil {
		t.Fatalf("Insert: %v", err)
	}
	if s, err := scoreOf(collCos, "far"); err != nil || !closeTo(s, 1.0) {
		t.Errorf("COSINE score(far) = %v, %v; want 1.0", s, err)
	}
	if s, err := scoreOf(collCos, "near"); err != nil || !closeTo(s, 1.5/math.Sqrt(2.5)) {
		t.Errorf("COSINE score(near) = %v, %v; want 1.5/sqrt(2.5)", s, err)
	}
	if s, err := scoreOf(collCos, "zero"); err != nil || s != 0.0 {
		t.Errorf("COSINE score(zero vector) = %v, %v; want exactly 0.0", s, err)
	}

	// Length mismatch: L2 yields -Inf (ranked last); COSINE yields
	// exactly 0.0.
	miscoll := newMutationColl(t) // "emb" is dim 4, metric defaults to L2
	dm := NewDocument("match")
	dm.SetVector("emb", []float32{2, 0, 0, 0})
	dd := NewDocument("wrongdim")
	dd.SetVector("emb", []float32{1, 1, 1})
	if err := miscoll.Insert(dm); err != nil || miscoll.Insert(dd) != nil {
		t.Fatalf("Insert: %v", err)
	}
	res, err := miscoll.Search(NewVectorQueryByVector("emb", []float32{1, 0, 0, 0}).WithTopK(5))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var match, wrong float64
	for _, r := range res {
		switch r.ID {
		case "match":
			match = r.Score
		case "wrongdim":
			wrong = r.Score
		}
	}
	if !closeTo(match, -1.0) {
		t.Errorf("L2 score(match) = %v, want -1.0", match)
	}
	if wrong != math.Inf(-1) {
		t.Errorf("L2 score(wrong dim) = %v, want -Inf", wrong)
	}
	if len(res) != 2 || res[0].ID != "match" {
		t.Errorf("Search with mismatched dim: order = %v, want match first", res)
	}

	// Cosine with a dimension mismatch is guarded by similarity() first:
	// it must score -Inf too (the 0.0 branch in cosineSimilarity itself
	// is unreachable through Search, which always checks lengths first).
	cosMiscoll := mk(MetricTypeCOSINE)
	dm2 := NewDocument("match")
	dm2.SetVector("emb", []float32{2, 0})
	dd2 := NewDocument("wrongdim")
	dd2.SetVector("emb", []float32{1, 1, 1})
	if err := cosMiscoll.Insert(dm2); err != nil || cosMiscoll.Insert(dd2) != nil {
		t.Fatalf("Insert: %v", err)
	}
	if s, err := scoreOf(cosMiscoll, "wrongdim"); err != nil || s != math.Inf(-1) {
		t.Errorf("COSINE score(wrong dim) = %v, %v; want -Inf", s, err)
	}
}

// TestStatsReflectsDisk kills the Stats ReadDir/Info branch mutants.
func TestStatsReflectsDisk(t *testing.T) {
	coll := newMutationColl(t)
	// Before any document: no docs dir yet; stats must still work.
	st, err := coll.Stats()
	if err != nil || st.DocCount != 0 || st.SizeBytes != 0 {
		t.Fatalf("Stats(empty) = %+v, %v; want 0/0", st, err)
	}
	if err := coll.Insert(NewDocument("s1").SetField("n", int64(42))); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	st, err = coll.Stats()
	if err != nil || st.DocCount != 1 || st.SizeBytes <= 0 {
		t.Fatalf("Stats(1 doc) = %+v, %v; want count=1 size>0", st, err)
	}
}

// TestFlushFailurePropagation kills the Flush writeDocument error
// mutants by making one rename target a directory.
func TestFlushFailurePropagation(t *testing.T) {
	coll := newMutationColl(t)
	docsDir := filepath.Join(coll.path, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "blocked.json"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Two in-memory docs: one writable, one blocked.
	coll.mu.Lock()
	coll.docs["fl1"] = NewDocument("fl1").SetField("n", int64(1))
	coll.docs["blocked"] = NewDocument("blocked").SetField("n", int64(1))
	coll.mu.Unlock()
	if err := coll.Flush(); err == nil {
		t.Error("Flush: want write failure (path is a directory), got nil")
	}
}

// TestWriteDocumentFailurePropagation kills the writeDocument error
// mutants across Upsert/UpsertBatch/InsertBatch/Update/AlterColumn/
// DropColumn and the writeDocument validation/marshal/MkdirAll/CreateTemp
// error branches. Rename-target-as-directory fails without root; each
// DDL scenario uses a freshly injected in-memory set so map iteration
// order cannot mask the failure.
func TestWriteDocumentFailurePropagation(t *testing.T) {
	mkBlocked := func() *Collection {
		coll := newMutationColl(t)
		docsDir := filepath.Join(coll.path, "docs")
		if err := os.MkdirAll(filepath.Join(docsDir, "blocked.json"), 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		return coll
	}
	injectN := func(coll *Collection, ids ...string) {
		coll.mu.Lock()
		defer coll.mu.Unlock()
		for _, id := range ids {
			coll.docs[id] = NewDocument(id).SetField("n", int64(1))
		}
	}

	// UpsertBatch: first doc persists, second hits the directory.
	coll := mkBlocked()
	n, err := coll.UpsertBatch([]*Document{
		NewDocument("ok1").SetField("n", int64(1)),
		NewDocument("blocked").SetField("n", int64(1)),
	})
	if err == nil || n != 1 {
		t.Errorf("UpsertBatch = %d, %v; want 1, err", n, err)
	}
	// The failed upsert still left the doc in the in-memory map.
	if _, ok := coll.docs["blocked"]; !ok {
		t.Fatal("blocked doc missing from map")
	}
	// UpsertBatch nil / invalid-id: a valid doc comes first so the
	// returned count (1) distinguishes the count->0 mutants.
	n, err = coll.UpsertBatch([]*Document{NewDocument("ubn1"), nil})
	if err == nil || n != 1 {
		t.Errorf("UpsertBatch(valid, nil) = %d, %v; want 1, err", n, err)
	}
	n, err = coll.UpsertBatch([]*Document{NewDocument("ubi1"), NewDocument("a/b")})
	if err == nil || n != 1 {
		t.Errorf("UpsertBatch(valid, bad-id) = %d, %v; want 1, err", n, err)
	}
	// Successful UpsertBatch count.
	n, err = coll.UpsertBatch([]*Document{
		NewDocument("ub2").SetField("n", int64(1)),
		NewDocument("ub3"),
	})
	if err != nil || n != 2 {
		t.Errorf("UpsertBatch(2 valid) = %d, %v; want 2, nil", n, err)
	}

	// Upsert error branches: nil doc, empty id, invalid id, blocked write.
	if err := coll.Upsert(nil); err == nil {
		t.Error("Upsert(nil): want error, got nil")
	}
	if err := coll.Upsert(&Document{ID: ""}); err == nil {
		t.Error("Upsert(empty id): want error, got nil")
	}
	if err := coll.Upsert(NewDocument("a/b")); err == nil {
		t.Error("Upsert(bad id): want error, got nil")
	}
	if err := coll.Upsert(NewDocument("blocked").SetField("n", int64(1))); err == nil {
		t.Error("Upsert(blocked path): want write failure, got nil")
	}

	// UpsertBatch after Close: (0, err).
	closedColl := newMutationColl(t)
	closedColl.Close()
	if n, err := closedColl.UpsertBatch([]*Document{NewDocument("x")}); err == nil || n != 0 {
		t.Errorf("UpsertBatch after Close = %d, %v; want 0, err", n, err)
	}

	// InsertBatch: second doc hits the blocked path.
	ib := mkBlocked()
	n, err = ib.InsertBatch([]*Document{
		NewDocument("ib1").SetField("n", int64(1)),
		NewDocument("blocked").SetField("n", int64(1)),
	})
	if err == nil || n != 1 {
		t.Errorf("InsertBatch = %d, %v; want 1, err", n, err)
	}

	// Update: rewrite of the blocked doc must fail.
	up := mkBlocked()
	if err := up.Update(NewDocument("blocked")); err == nil {
		t.Error("Update: want write failure, got nil")
	}

	// AlterColumn: three injected docs (one blocked) all carry the field;
	// whichever order the loop takes, the blocked write must fail.
	ac := mkBlocked()
	injectN(ac, "da", "db", "blocked")
	if err := ac.AlterColumn("n", "n2", nil, nil); err == nil {
		t.Error("AlterColumn: want write failure, got nil")
	}

	// DropColumn: same injection; scrubbing must hit the blocked doc.
	dc := mkBlocked()
	injectN(dc, "da", "db", "blocked")
	if err := dc.DropColumn("n"); err == nil {
		t.Error("DropColumn: want write failure, got nil")
	}

	// Direct writeDocument error branches (same package).
	if err := coll.writeDocument(&Document{ID: "a/b"}); err == nil {
		t.Error("writeDocument(bad id): want error, got nil")
	}
	bad := NewDocument("um")
	bad.SetField("c", make(chan int)) // unmarshalable value
	if err := coll.writeDocument(bad); err == nil {
		t.Error("writeDocument(unmarshalable): want error, got nil")
	}

	// MkdirAll failure: "docs" is a regular file.
	md := newMutationColl(t)
	if err := os.WriteFile(filepath.Join(md.path, "docs"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := md.Insert(NewDocument("md")); err == nil {
		t.Error("Insert(docs is a file): want MkdirAll failure, got nil")
	}

	// CreateTemp failure: docs dir exists but is read-only.
	ro := newMutationColl(t)
	if err := ro.Insert(NewDocument("ro0")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	roDir := filepath.Join(ro.path, "docs")
	if err := os.Chmod(roDir, 0500); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0755) })
	if err := ro.Insert(NewDocument("ro1")); err == nil {
		t.Error("Insert(read-only docs dir): want CreateTemp failure, got nil")
	}
}

// TestAddColumnErrorBranches kills the AddColumn validation / duplicate
// field / persistSchema error mutants.
func TestAddColumnErrorBranches(t *testing.T) {
	coll := newMutationColl(t) // has scalar "n" and vector "emb"
	if err := coll.AddColumn(nil, "", nil); err == nil {
		t.Error("AddColumn(nil schema): want error, got nil")
	}
	if err := coll.AddColumn(NewFieldSchema("f", DataTypeVectorFP32), "", nil); err == nil {
		t.Error("AddColumn(vector type as scalar): want error, got nil")
	}
	if err := coll.AddColumn(NewFieldSchema("n", DataTypeInt64), "", nil); err == nil {
		t.Error("AddColumn(duplicate scalar): want error, got nil")
	}
	if err := coll.AddColumn(NewFieldSchema("emb", DataTypeInt64), "", nil); err == nil {
		t.Error("AddColumn(duplicate vector name): want error, got nil")
	}
	// Successful add must persist the schema.
	if err := coll.AddColumn(NewFieldSchema("c2", DataTypeInt64), "", nil); err != nil {
		t.Fatalf("AddColumn(valid): %v", err)
	}
	found := false
	for _, f := range coll.schema.Fields {
		if f.Name == "c2" {
			found = true
		}
	}
	if !found {
		t.Error("AddColumn(valid): c2 missing from schema")
	}

	// persistSchema failure: collection.json is a directory.
	coll2 := newMutationColl(t)
	// Replace the metadata file with a directory so persistSchema's
	// rename fails.
	if err := os.Remove(filepath.Join(coll2.path, "collection.json")); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(coll2.path, "collection.json"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := coll2.AddColumn(NewFieldSchema("c3", DataTypeInt64), "", nil); err == nil {
		t.Error("AddColumn with collection.json as directory: want persist failure, got nil")
	}
}

// TestAlterColumnErrorBranches kills the AlterColumn duplicate-name,
// unknown-field, and self-rename guard mutants.
func TestAlterColumnErrorBranches(t *testing.T) {
	coll := newMutationCollSchema(t, func() *CollectionSchema {
		schema := NewCollectionSchema("ac")
		schema.AddField(NewFieldSchema("n", DataTypeInt64))
		schema.AddField(NewFieldSchema("n2", DataTypeInt64))
		schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 2))
		return schema
	})
	if err := coll.AlterColumn("n", "n2", nil, nil); err == nil {
		t.Error("AlterColumn(rename to existing scalar): want error, got nil")
	}
	if err := coll.AlterColumn("n", "emb", nil, nil); err == nil {
		t.Error("AlterColumn(rename to vector name): want error, got nil")
	}
	if err := coll.AlterColumn("nope", "x", nil, nil); err == nil {
		t.Error("AlterColumn(unknown old name): want error, got nil")
	}
	// Renaming a column to its own name is a no-op and must succeed
	// (kills the guard's && -> || mutant, which would treat the field
	// as a duplicate).
	if err := coll.AlterColumn("n", "n", nil, nil); err != nil {
		t.Errorf("AlterColumn(self rename): %v, want nil", err)
	}
}

// TestFetchDiskAndInMemory kills the Fetch in-memory/disk branch
// mutants: in-memory wins when present, disk is the fallback, and a
// disk-only doc must be returned non-nil.
func TestFetchDiskAndInMemory(t *testing.T) {
	coll := newMutationColl(t)
	d1 := NewDocument("f1")
	d1.SetField("n", int64(3))
	if err := coll.Insert(d1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// f1: removed from memory so only the disk copy remains.
	coll.mu.Lock()
	delete(coll.docs, "f1")
	coll.mu.Unlock()
	// f2: modified in memory after persistence; memory must win.
	d2 := NewDocument("f2")
	d2.SetField("n", int64(5))
	if err := coll.Insert(d2); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	d2.SetField("n", int64(9))

	got, err := coll.Fetch([]string{"f1", "f2", "missing"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	f1, ok := got["f1"]
	if !ok || f1 == nil {
		t.Fatal("Fetch: disk-only doc f1 missing or nil")
	}
	if v, _ := f1.Fields["n"]; v != float64(3) && v != int64(3) {
		t.Errorf("f1.n = %v, want 3 (disk value)", v)
	}
	f2, ok := got["f2"]
	if !ok || f2 == nil {
		t.Fatal("Fetch: in-memory doc f2 missing or nil")
	}
	if v, _ := f2.Fields["n"]; v != int64(9) {
		t.Errorf("f2.n = %v, want 9 (in-memory value)", v)
	}
	if _, ok := got["missing"]; ok {
		t.Error("Fetch: missing id should be omitted")
	}
}

// TestLoadDocsSkipsCorruptAndInfersID kills the loadDocs error/skip
// mutants: corrupt files and subdirectories are skipped, readable docs
// load, and missing ids are inferred from file names.
func TestLoadDocsSkipsCorruptAndInfersID(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	schema := NewCollectionSchema("ld")
	schema.AddField(NewFieldSchema("n", DataTypeInt64))
	dir := filepath.Join(t.TempDir(), "ld")
	coll, err := CreateAndOpen(dir, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	d := NewDocument("good")
	d.SetField("n", int64(7))
	if err := coll.Insert(d); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Corrupt file, a stray subdirectory, and a file with no id field
	// (id must be inferred from the file name).
	docsDir := filepath.Join(dir, "docs")
	os.WriteFile(filepath.Join(docsDir, "corrupt.json"), []byte("{{{"), 0644)
	os.MkdirAll(filepath.Join(docsDir, "subdir"), 0755)
	type rawDoc struct {
		Fields map[string]interface{} `json:"fields"`
	}
	rd := rawDoc{Fields: map[string]interface{}{"n": 9}}
	blob, _ := json.Marshal(rd)
	os.WriteFile(filepath.Join(docsDir, "fromname.json"), blob, 0644)
	coll.Close()

	reopened, err := Open(dir, nil)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer reopened.Close()
	if _, err := reopened.Get("good"); err != nil {
		t.Errorf("good doc missing after reload: %v", err)
	}
	got, err := reopened.Get("fromname")
	if err != nil {
		t.Fatalf("fromname not loaded: %v", err)
	}
	if v, _ := got.Fields["n"]; v != float64(9) && v != int64(9) {
		t.Errorf("fromname.n = %v, want 9", v)
	}
	if _, err := reopened.Get("corrupt"); !errors.Is(err, ErrDocNotFound) {
		t.Errorf("corrupt doc Get = %v, want ErrDocNotFound", err)
	}
	// ListIDs distinguishes the skip/inference mutants: Get alone cannot,
	// because it falls back to reading the raw file.
	ids, err := reopened.ListIDs()
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	set := make(map[string]bool)
	for _, id := range ids {
		set[id] = true
	}
	if !set["good"] || !set["fromname"] {
		t.Errorf("reloaded ListIDs = %v; want good and fromname", ids)
	}
	if set[""] || set["corrupt"] {
		t.Errorf("reloaded ListIDs = %v; must not contain empty/corrupt ids", ids)
	}
}

// TestOpenFailsWhenDocsNotADir kills the loadDocs ReadDir error-return
// mutants (including the IsNotExist branch) and the zvec.Open error
// return: a regular file named "docs" makes ReadDir fail with a
// non-NotExist error, which must propagate as an Open failure.
// (Corrupt doc files, by contrast, are intentionally skipped.)
func TestOpenFailsWhenDocsNotADir(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	schema := NewCollectionSchema("od")
	schema.AddField(NewFieldSchema("n", DataTypeInt64))
	dir := filepath.Join(t.TempDir(), "od")
	if _, err := CreateAndOpen(dir, schema, nil); err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	// Replace the (absent) docs directory with a regular file.
	if err := os.WriteFile(filepath.Join(dir, "docs"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if c, err := Open(dir, nil); err == nil || c != nil {
		t.Errorf("Open with docs as a regular file: c=%v err=%v; want nil, err", c, err)
	}
}

// TestOpenErrorPaths kills the zvec.Open error-path mutants:
// uninitialized instance, missing collection, unreadable metadata, and
// corrupt metadata. Every error path must return a nil *Collection.
func TestOpenErrorPaths(t *testing.T) {
	// Uninitialized Zvec instance.
	z := &Zvec{}
	if c, err := z.Open("/nonexistent-collection-x", nil); err == nil || c != nil {
		t.Errorf("Open on uninitialized zvec: c=%v err=%v; want nil, err", c, err)
	}

	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Missing collection directory.
	if c, err := Open(filepath.Join(t.TempDir(), "nope"), nil); err == nil || c != nil {
		t.Errorf("Open(missing path): c=%v err=%v; want nil, err", c, err)
	}

	// collection.json is a directory: stat succeeds, ReadFile fails.
	dir := filepath.Join(t.TempDir(), "asdir")
	if err := os.MkdirAll(filepath.Join(dir, "collection.json"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if c, err := Open(dir, nil); err == nil || c != nil {
		t.Errorf("Open(collection.json is a directory): c=%v err=%v; want nil, err", c, err)
	}

	// Corrupt collection.json.
	dir2 := filepath.Join(t.TempDir(), "bad")
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "collection.json"), []byte("!!!"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if c, err := Open(dir2, nil); err == nil || c != nil {
		t.Errorf("Open(corrupt metadata): c=%v err=%v; want nil, err", c, err)
	}
}

// TestCreateAndOpenErrorPaths kills the CreateAndOpen error-path
// mutants: uninitialized instance, nil schema, uncreatable directory,
// unwritable metadata, unmarshalable schema, and the default-option
// fallback. Every error path must return a nil *Collection.
func TestCreateAndOpenErrorPaths(t *testing.T) {
	// Before Init: GetInstance must fail with a nil instance, and the
	// package-level calls must fail cleanly.
	globalZvec = nil
	once = sync.Once{}
	if z, err := GetInstance(); err == nil || z != nil {
		t.Errorf("GetInstance before Init: z=%v err=%v; want nil, err", z, err)
	}
	if c, err := CreateAndOpen("/nonexistent-x", NewCollectionSchema("x"), nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen before Init: c=%v err=%v; want nil, err", c, err)
	}
	if c, err := Open("/nonexistent-x", nil); err == nil || c != nil {
		t.Errorf("Open before Init: c=%v err=%v; want nil, err", c, err)
	}

	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Uninitialized Zvec instance.
	z := &Zvec{}
	if c, err := z.CreateAndOpen(t.TempDir(), NewCollectionSchema("x"), nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen on uninitialized zvec: c=%v err=%v; want nil, err", c, err)
	}

	// Nil schema.
	if c, err := CreateAndOpen(t.TempDir(), nil, nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen(nil schema): c=%v err=%v; want nil, err", c, err)
	}

	// Path parent is a regular file: MkdirAll fails.
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "blk"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if c, err := CreateAndOpen(filepath.Join(base, "blk", "child"), NewCollectionSchema("x"), nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen(parent is a file): c=%v err=%v; want nil, err", c, err)
	}

	// collection.json is a directory: the metadata WriteFile fails.
	dir := filepath.Join(t.TempDir(), "asdir")
	if err := os.MkdirAll(filepath.Join(dir, "collection.json"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if c, err := CreateAndOpen(dir, NewCollectionSchema("x"), nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen(collection.json is a directory): c=%v err=%v; want nil, err", c, err)
	}

	// Unmarshalable schema (chan as index param): metadata marshal fails.
	schema := NewCollectionSchema("um")
	vs := NewVectorSchema("v", DataTypeVectorFP32, 2)
	vs.IndexParam = make(chan int)
	schema.AddVectorField(vs)
	if c, err := CreateAndOpen(t.TempDir(), schema, nil); err == nil || c != nil {
		t.Errorf("CreateAndOpen(unmarshalable schema): c=%v err=%v; want nil, err", c, err)
	}

	// Nil option must be replaced by the default (non-nil).
	coll, err := CreateAndOpen(t.TempDir(), NewCollectionSchema("opt"), nil)
	if err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	defer coll.Close()
	if coll.option == nil || !coll.option.CreateIfMissing {
		t.Errorf("CreateAndOpen(nil option): option = %+v, want non-nil default", coll.option)
	}
}

// TestInitNilAndFileLogging kills the Init nil-config, log-branch and
// log-directory error mutants, and pins the DefaultConfig values.
func TestInitNilAndFileLogging(t *testing.T) {
	const relLogDir = "./logs"
	if err := os.RemoveAll(relLogDir); err != nil {
		t.Fatalf("cleaning %s: %v", relLogDir, err)
	}
	// A default (console) config must never create the log directory;
	// clean it up afterwards if a mutant or side effect created it.
	defer func() {
		if _, err := os.Stat(relLogDir); err == nil {
			os.RemoveAll(relLogDir)
		}
	}()

	globalZvec = nil
	once = sync.Once{}
	if err := Init(nil); err != nil {
		t.Fatalf("Init(nil): %v", err)
	}
	z, err := GetInstance()
	if err != nil || z == nil {
		t.Fatalf("GetInstance after Init(nil): %v, %v", z, err)
	}
	cfg := z.Config()
	if cfg == nil ||
		cfg.LogType != LogTypeConsole ||
		cfg.LogLevel != LogLevelWarn ||
		cfg.LogDir != "./logs" ||
		cfg.LogBasename != "zvec.log" ||
		cfg.LogFileSize != 2048 ||
		cfg.LogOverdueDays != 7 {
		t.Fatalf("Init(nil) defaults: %+v", cfg)
	}
	if _, err := os.Stat(relLogDir); err == nil {
		t.Error("default console config must not create the log directory")
	}

	globalZvec = nil
	once = sync.Once{}
	logDir := filepath.Join(t.TempDir(), "logs")
	if err := Init(&Config{LogType: LogTypeFile, LogDir: logDir}); err != nil {
		t.Fatalf("Init(file): %v", err)
	}
	if _, err := os.Stat(logDir); err != nil {
		t.Errorf("log dir not created: %v", err)
	}

	// LogTypeFile with empty LogDir: no directory creation, no error.
	globalZvec = nil
	once = sync.Once{}
	if err := Init(&Config{LogType: LogTypeFile, LogDir: ""}); err != nil {
		t.Errorf("Init(file, empty dir): %v", err)
	}

	// LogTypeFile with an uncreatable LogDir must surface an error.
	globalZvec = nil
	once = sync.Once{}
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "blk"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := Init(&Config{LogType: LogTypeFile, LogDir: filepath.Join(base, "blk", "sub")}); err == nil {
		t.Error("Init(file, uncreatable dir): want error, got nil")
	}
}

// TestCreateAndOpenOptionHandling kills the CreateAndOpen/Open option
// resolution mutants (caller option wins; persisted option restored).
func TestCreateAndOpenOptionHandling(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}
	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	schema := NewCollectionSchema("opt")
	schema.AddField(NewFieldSchema("n", DataTypeInt64))
	dir := filepath.Join(t.TempDir(), "opt")

	// Caller option is persisted and restored on reopen with nil.
	ro := &CollectionOption{ReadOnly: true, CreateIfMissing: true, ErrorIfExists: false}
	c1, err := CreateAndOpen(dir, schema, ro)
	if err != nil {
		t.Fatalf("CreateAndOpen: %v", err)
	}
	if c1.option != ro {
		t.Errorf("CreateAndOpen: option = %+v, want the caller's", c1.option)
	}
	c1.Close()
	c2, err := Open(dir, nil)
	if err != nil {
		t.Fatalf("Open(nil): %v", err)
	}
	if c2.option == nil || !c2.option.ReadOnly {
		t.Errorf("reopened option = %+v, want persisted ReadOnly=true", c2.option)
	}
	// Explicit caller option wins over the persisted one.
	c3, err := Open(dir, DefaultCollectionOption())
	if err != nil {
		t.Fatalf("Open(default): %v", err)
	}
	if c3.option == nil || c3.option.ReadOnly {
		t.Errorf("Open with explicit option: %+v, caller option must win", c3.option)
	}
	c2.Close()
	c3.Close()
}

// TestStatusAndTypeMarshalling kills the Status branch mutants and the
// custom UnmarshalJSON / String mutants for all string-typed enums.
func TestStatusAndTypeMarshalling(t *testing.T) {
	ok := Status{Code: StatusCodeOK}
	if !ok.IsOK() || ok.Error() != "OK" {
		t.Errorf("Status OK: isok=%v err=%q", ok.IsOK(), ok.Error())
	}
	bad := Status{Code: StatusCodeNotFound, Message: "nope"}
	if bad.IsOK() || bad.Error() == "" || !strings.Contains(bad.Error(), "nope") {
		t.Errorf("Status NotFound: %q", bad.Error())
	}
	if bad.String() != bad.Error() {
		t.Errorf("Status.String() = %q, want %q", bad.String(), bad.Error())
	}

	// Valid values must unmarshal into the right constants.
	var lt LogType
	if err := json.Unmarshal([]byte(`"FILE"`), &lt); err != nil || lt != LogTypeFile {
		t.Errorf("LogType from FILE: %v, %v", lt, err)
	}
	var lv LogLevel
	if err := json.Unmarshal([]byte(`"DEBUG"`), &lv); err != nil || lv != LogLevelDebug {
		t.Errorf("LogLevel from DEBUG: %v, %v", lv, err)
	}
	var dt DataType
	if err := json.Unmarshal([]byte(`"INT64"`), &dt); err != nil || dt != DataTypeInt64 {
		t.Errorf("DataType from INT64: %v, %v", dt, err)
	}
	var st State
	if err := json.Unmarshal([]byte(`"completed"`), &st); err != nil || st != StateCompleted {
		t.Errorf("State from completed: %v, %v", st, err)
	}
	var mt MetricType
	if err := json.Unmarshal([]byte(`"L2"`), &mt); err != nil || mt != MetricTypeL2 {
		t.Errorf("MetricType from L2: %v, %v", mt, err)
	}

	// Non-string JSON must be rejected by every unmarshaler.
	if err := json.Unmarshal([]byte(`123`), &dt); err == nil {
		t.Error("DataType from number: want error, got nil")
	}
	if err := json.Unmarshal([]byte(`123`), &lt); err == nil {
		t.Error("LogType from number: want error, got nil")
	}
	if err := json.Unmarshal([]byte(`123`), &lv); err == nil {
		t.Error("LogLevel from number: want error, got nil")
	}
	if err := json.Unmarshal([]byte(`123`), &st); err == nil {
		t.Error("State from number: want error, got nil")
	}
	if err := json.Unmarshal([]byte(`123`), &mt); err == nil {
		t.Error("MetricType from number: want error, got nil")
	}

	// String() must round-trip the constant values.
	if LogTypeFile.String() != "FILE" || LogTypeConsole.String() != "CONSOLE" {
		t.Error("LogType.String() mismatch")
	}
	if LogLevelDebug.String() != "DEBUG" {
		t.Error("LogLevel.String() mismatch")
	}
	if DataTypeInt64.String() != "INT64" {
		t.Error("DataType.String() mismatch")
	}
	if StateCompleted.String() != "completed" {
		t.Error("State.String() mismatch")
	}
	if MetricTypeL2.String() != "L2" {
		t.Error("MetricType.String() mismatch")
	}
}

// TestSchemaBuilderAndValidateMutants kills the schema builder/validate
// mutants.
func TestSchemaBuilderAndValidateMutants(t *testing.T) {
	// Builder returns the same schema (chainable).
	s := NewCollectionSchema("chain")
	if s.AddField(NewFieldSchema("a", DataTypeInt64)) != s {
		t.Error("AddField must return the same schema")
	}
	if s.AddVectorField(NewVectorSchema("v", DataTypeVectorFP32, 2)) != s {
		t.Error("AddVectorField must return the same schema")
	}

	// A valid schema must validate cleanly (kills && vs || mutants in
	// the dimension check and per-field validate skip mutants).
	valid := NewCollectionSchema("valid")
	valid.AddField(NewFieldSchema("a", DataTypeInt64))
	valid.AddVectorField(NewVectorSchema("v", DataTypeVectorFP32, 2))
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate(valid schema) = %v, want nil", err)
	}

	// Empty collection name must fail.
	if err := NewCollectionSchema("").Validate(); err == nil {
		t.Error("Validate(empty name): want error")
	}

	// Nil field entries must fail.
	ne := NewCollectionSchema("ne")
	ne.AddField(nil)
	if err := ne.Validate(); err == nil {
		t.Error("Validate(nil scalar field): want error")
	}
	ne2 := NewCollectionSchema("ne2")
	ne2.AddVectorField(nil)
	if err := ne2.Validate(); err == nil {
		t.Error("Validate(nil vector field): want error")
	}

	// Duplicate scalar field names must fail.
	dupSc := NewCollectionSchema("dupsc")
	dupSc.AddField(NewFieldSchema("same", DataTypeInt64))
	dupSc.AddField(NewFieldSchema("same", DataTypeString))
	if err := dupSc.Validate(); err == nil {
		t.Error("Validate(duplicate scalar names): want error")
	}

	// Invalid scalar field (vector data type) must fail schema validation.
	bad := NewCollectionSchema("bad")
	bad.AddField(NewFieldSchema("f", DataTypeVectorFP32))
	if err := bad.Validate(); err == nil {
		t.Error("Validate: scalar field with vector type must fail")
	}

	// Invalid vector field (scalar data type) must fail.
	badv := NewCollectionSchema("badv")
	badv.AddVectorField(NewVectorSchema("f", DataTypeInt64, 2))
	if err := badv.Validate(); err == nil {
		t.Error("Validate: vector field with scalar type must fail")
	}

	// Field with empty name must fail.
	empty := NewCollectionSchema("empty")
	empty.AddField(&FieldSchema{Name: "", DataType: DataTypeInt64})
	if err := empty.Validate(); err == nil {
		t.Error("Validate: empty field name must fail")
	}

	// Scalar and vector sharing a name must fail.
	dup := NewCollectionSchema("dup")
	dup.AddField(NewFieldSchema("same", DataTypeInt64))
	dup.AddVectorField(NewVectorSchema("same", DataTypeVectorFP32, 2))
	if err := dup.Validate(); err == nil {
		t.Error("Validate: duplicate scalar/vector name must fail")
	}

	// Dense vector with dimension 0 must fail.
	dv := NewVectorSchema("dense", DataTypeVectorFP32, 0)
	if err := dv.Validate(); err == nil {
		t.Error("Validate: dense vector dim 0 must fail")
	}
	// Sparse vectors may have dimension 0.
	sv := NewVectorSchema("sparse", DataTypeSparseVectorFP32, 0)
	if err := sv.Validate(); err != nil {
		t.Errorf("Validate: sparse vector dim 0 should be OK, got %v", err)
	}
	// Dense vector with positive dimension is fine.
	okv := NewVectorSchema("ok", DataTypeVectorFP32, 2)
	if err := okv.Validate(); err != nil {
		t.Errorf("Validate: dense vector dim 2 should be OK, got %v", err)
	}
	// dim 1 must be accepted (kills the dim<=0 -> dim<=1 constant mutant).
	if err := NewVectorSchema("dim1", DataTypeVectorFP32, 1).Validate(); err != nil {
		t.Errorf("Validate: dense vector dim 1 should be OK, got %v", err)
	}
}

// TestSchemaConstructorsAndDefaults pins the default field values of
// every param/option/query constructor (killing zero-value ReturnVals
// and constant mutants) and the VectorQuery helpers.
func TestSchemaConstructorsAndDefaults(t *testing.T) {
	// Index param constructors.
	if p := NewInvertIndexParam(); p == nil || p.EnableRangeOptimization {
		t.Errorf("NewInvertIndexParam = %+v, want non-nil, disabled", p)
	}
	if p := NewHnswIndexParam(); p == nil || p.M != 16 || p.EfConstruction != 200 || p.EfSearch != 128 {
		t.Errorf("NewHnswIndexParam = %+v, want M=16 EfConstruction=200 EfSearch=128", p)
	}
	if p := NewIVFIndexParam(); p == nil || p.NList != 1024 || p.NProbe != 8 {
		t.Errorf("NewIVFIndexParam = %+v, want NList=1024 NProbe=8", p)
	}
	if p := NewFlatIndexParam(); p == nil || p.MetricType != MetricTypeL2 {
		t.Errorf("NewFlatIndexParam = %+v, want L2 metric", p)
	}

	// Query param constructors.
	if q := NewHnswQueryParam(); q == nil || q.Ef != 128 {
		t.Errorf("NewHnswQueryParam = %+v, want Ef=128", q)
	}
	if q := NewIVFQueryParam(); q == nil || q.NProbe != 8 {
		t.Errorf("NewIVFQueryParam = %+v, want NProbe=8", q)
	}

	// Vector query constructors pin TopK and the carried values.
	qv := NewVectorQueryByVector("emb", []float32{1, 0})
	if qv == nil || qv.FieldName != "emb" || qv.TopK != 10 || len(qv.Vector) != 2 {
		t.Errorf("NewVectorQueryByVector = %+v, want emb/TopK=10/vector", qv)
	}
	qi := NewVectorQueryByID("emb", "doc1")
	if qi == nil || qi.FieldName != "emb" || qi.ID != "doc1" || qi.TopK != 10 {
		t.Errorf("NewVectorQueryByID = %+v, want emb/doc1/TopK=10", qi)
	}
	if !qi.HasID() || qi.HasVector() {
		t.Error("HasID/HasVector wrong for ID query")
	}
	if qv.HasID() || !qv.HasVector() {
		t.Error("HasID/HasVector wrong for vector query")
	}
	// A single-element vector is still a vector (kills len>0 -> len>1).
	one := NewVectorQueryByVector("emb", []float32{1})
	if !one.HasVector() {
		t.Error("1-element vector must be HasVector")
	}

	// Validate branches on VectorQuery.
	if err := (&VectorQuery{}).Validate(); err == nil {
		t.Error("Validate(empty field name): want error")
	}
	if err := (&VectorQuery{FieldName: "emb", ID: "x", Vector: []float32{1}}).Validate(); err == nil {
		t.Error("Validate(id and vector): want error")
	}
	if err := (&VectorQuery{FieldName: "emb"}).Validate(); err == nil {
		t.Error("Validate(neither id nor vector): want error")
	}
	if err := qv.Validate(); err != nil {
		t.Errorf("Validate(valid query): %v", err)
	}

	// Option defaults.
	if o := DefaultIndexOption(); o == nil || o.Async {
		t.Errorf("DefaultIndexOption = %+v, want non-nil, async=false", o)
	}
	if o := DefaultOptimizeOption(); o == nil || o.Full {
		t.Errorf("DefaultOptimizeOption = %+v, want non-nil, full=false", o)
	}
	if o := DefaultAddColumnOption(); o == nil || o.SkipBackfill {
		t.Errorf("DefaultAddColumnOption = %+v, want non-nil, skip=false", o)
	}
	if o := DefaultAlterColumnOption(); o == nil || o.SkipReindex {
		t.Errorf("DefaultAlterColumnOption = %+v, want non-nil, skip=false", o)
	}
	if o := DefaultCollectionOption(); o == nil || o.ReadOnly || !o.CreateIfMissing || o.ErrorIfExists {
		t.Errorf("DefaultCollectionOption = %+v, want RO=false CIM=true EIF=false", o)
	}

	// With* methods must chain (return the same pointer).
	h := NewHnswIndexParam()
	if h.WithM(8).WithEfConstruction(50).WithEfSearch(30) != h {
		t.Error("HnswIndexParam With* must chain")
	}
	v := NewVectorSchema("v", DataTypeVectorFP32, 2)
	if v.WithMetricType(MetricTypeIP).WithIndexParam(NewHnswQueryParam()) != v {
		t.Error("VectorSchema With* must chain")
	}
	f := NewFieldSchema("f", DataTypeInt64)
	if f.WithNullable(true).WithIndexParam(NewInvertIndexParam()) != f {
		t.Error("FieldSchema With* must chain")
	}
	if o := DefaultIndexOption().WithAsync(true); o == nil || !o.Async {
		t.Error("IndexOption.WithAsync must set and chain")
	}
	if o := DefaultOptimizeOption().WithFull(true); o == nil || !o.Full {
		t.Error("OptimizeOption.WithFull must set and chain")
	}
	if o := DefaultAddColumnOption().WithSkipBackfill(true); o == nil || !o.SkipBackfill {
		t.Error("AddColumnOption.WithSkipBackfill must set and chain")
	}
	if o := DefaultAlterColumnOption().WithSkipReindex(true); o == nil || !o.SkipReindex {
		t.Error("AlterColumnOption.WithSkipReindex must set and chain")
	}
	if p := NewInvertIndexParam().WithEnableRangeOptimization(true); p == nil || !p.EnableRangeOptimization {
		t.Error("InvertIndexParam.WithEnableRangeOptimization must set and chain")
	}
	if q := NewHnswQueryParam().WithEf(256); q == nil || q.Ef != 256 {
		t.Error("HnswQueryParam.WithEf must set and chain")
	}
	if q := NewIVFQueryParam().WithNProbe(16); q == nil || q.NProbe != 16 {
		t.Error("IVFQueryParam.WithNProbe must set and chain")
	}
}

// TestQueryExecutorErrorPath kills the QueryExecutor.Execute error
// return mutants (bad query must surface the validation error).
func TestQueryExecutorErrorPath(t *testing.T) {
	coll := newMutationColl(t)
	schema := NewCollectionSchema("qe")
	schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 4))
	exec := NewQueryExecutor(schema)
	if exec == nil {
		t.Fatal("NewQueryExecutor returned nil")
	}
	ctx := &QueryContext{Query: &VectorQuery{}} // invalid: no field name
	res, err := exec.Execute(ctx, coll)
	if err == nil || res != nil {
		t.Errorf("Execute(invalid query) = %v, %v; want nil, err", res, err)
	}
}

// TestCosineSimilarityDirectLengthMismatch covers cosineSimilarity's own
// length-mismatch branch by calling the function directly. Search/Query
// cannot reach that branch (similarity() rejects mismatched lengths first),
// but a direct call makes the 0.0 constant observable.
func TestCosineSimilarityDirectLengthMismatch(t *testing.T) {
	if got := cosineSimilarity([]float32{1, 0}, []float32{1, 0, 0}); got != 0.0 {
		t.Errorf("cosineSimilarity(length mismatch) = %v, want 0.0", got)
	}
	if got := cosineSimilarity([]float32{1, 0}, []float32{0, 1}); got != 0.0 {
		t.Errorf("cosineSimilarity(orthogonal) = %v, want 0.0", got)
	}
	if got := cosineSimilarity([]float32{1, 0}, []float32{2, 0}); got != 1.0 {
		t.Errorf("cosineSimilarity(same direction) = %v, want 1.0", got)
	}
}

// TestAlterColumnReportsPersistFailure kills the ReturnVals mutant on
// AlterColumn's final "return c.persistSchema()": the per-document
// rewrites succeed, but the schema file rewrite fails because
// collection.json is blocked by a non-empty directory, so the error must
// be reported instead of swallowed.
func TestAlterColumnReportsPersistFailure(t *testing.T) {
	coll := newMutationColl(t)
	doc := NewDocument("a1").SetField("n", int64(1))
	if err := coll.Insert(doc); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	block := filepath.Join(coll.path, "collection.json")
	if err := os.Remove(block); err != nil {
		t.Fatalf("remove collection.json: %v", err)
	}
	if err := os.MkdirAll(block, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(block, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(block) })
	if err := coll.AlterColumn("n", "renamed", nil, nil); err == nil {
		t.Error("AlterColumn with blocked collection.json: expected error, got nil")
	}
}
