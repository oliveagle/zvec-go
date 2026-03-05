package zvec

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDocument(t *testing.T) {
	doc := NewDocument("doc1")
	if doc.ID != "doc1" {
		t.Errorf("Expected ID 'doc1', got '%s'", doc.ID)
	}

	// Test field
	doc.SetField("title", "Test Title")
	val, ok := doc.GetField("title")
	if !ok {
		t.Error("Expected field 'title' to exist")
	}
	if val != "Test Title" {
		t.Errorf("Expected 'Test Title', got %v", val)
	}

	// Test vector
	vec := []float32{1.0, 2.0, 3.0}
	doc.SetVector("embedding", vec)
	retVec, ok := doc.GetVector("embedding")
	if !ok {
		t.Error("Expected vector 'embedding' to exist")
	}
	if len(retVec) != 3 {
		t.Errorf("Expected vector length 3, got %d", len(retVec))
	}

	// Test metadata
	doc.SetMetadata("author", "test")
}

func TestCollection(t *testing.T) {
	// Reset global state
	globalZvec = nil
	once = sync.Once{}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize zvec
	cfg := DefaultConfig()
	cfg.LogDir = filepath.Join(tmpDir, "logs")
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create schema
	schema := NewCollectionSchema("test_collection")
	schema.AddField(NewFieldSchema("id", DataTypeInt64))
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	// Create and open collection
	collPath := filepath.Join(tmpDir, "test_coll")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	if coll.Path() != collPath {
		t.Errorf("Expected path %s, got %s", collPath, coll.Path())
	}

	if coll.Closed() {
		t.Error("Expected collection to be open")
	}
}

func TestCollectionInsertAndGet(t *testing.T) {
	// Reset global state
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddField(NewFieldSchema("id", DataTypeInt64))
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert document
	doc := NewDocument("doc1")
	doc.SetField("title", "Hello")
	doc.SetVector("embedding", []float32{1.0, 0.0, 0.0})

	if err := coll.Insert(doc); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Get document
	retDoc, err := coll.Get("doc1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retDoc.ID != "doc1" {
		t.Errorf("Expected doc ID 'doc1', got '%s'", retDoc.ID)
	}
}

func TestCollectionSearch(t *testing.T) {
	// Reset global state
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert test documents
	docs := []*Document{
		NewDocument("doc1").SetVector("embedding", []float32{1.0, 0.0, 0.0}),
		NewDocument("doc2").SetVector("embedding", []float32{0.0, 1.0, 0.0}),
		NewDocument("doc3").SetVector("embedding", []float32{0.0, 0.0, 1.0}),
	}

	for _, doc := range docs {
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Search
	query := NewVectorQueryByVector("embedding", []float32{1.0, 0.0, 0.0}).WithTopK(3)
	results, err := coll.Search(query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results")
	}
}

func TestCollectionCount(t *testing.T) {
	// Reset global state
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	count, err := coll.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Insert some docs
	for i := 0; i < 5; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	count, err = coll.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}
}

func TestCollectionDestroy(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}

	// Insert a doc
	doc := NewDocument("doc1")
	if err := coll.Insert(doc); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Destroy collection
	if err := coll.Destroy(); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	// Verify collection is closed
	if !coll.Closed() {
		t.Error("Expected collection to be closed after Destroy")
	}

	// Verify directory is removed
	if _, err := os.Stat(collPath); !os.IsNotExist(err) {
		t.Error("Expected collection directory to be removed")
	}
}

func TestCollectionFlush(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert docs
	for i := 0; i < 3; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Flush
	if err := coll.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify files exist
	docsDir := filepath.Join(collPath, "docs")
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		t.Error("Expected docs directory to exist after Flush")
	}
}

func TestCollectionStats(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Get stats (empty)
	stats, err := coll.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.DocCount != 0 {
		t.Errorf("Expected DocCount=0, got %d", stats.DocCount)
	}

	// Insert docs
	for i := 0; i < 5; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Get stats (with docs)
	stats, err = coll.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.DocCount != 5 {
		t.Errorf("Expected DocCount=5, got %d", stats.DocCount)
	}
}

func TestCollectionCreateIndex(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddField(NewFieldSchema("id", DataTypeInt64))
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Create index on vector field
	hnswParam := NewHnswIndexParam().WithM(16).WithEfConstruction(200)
	if err := coll.CreateIndex("embedding", hnswParam, DefaultIndexOption()); err != nil {
		t.Fatalf("CreateIndex failed: %v", err)
	}

	// Create index on scalar field
	invertParam := NewInvertIndexParam().WithEnableRangeOptimization(true)
	if err := coll.CreateIndex("id", invertParam, DefaultIndexOption()); err != nil {
		t.Fatalf("CreateIndex for scalar field failed: %v", err)
	}

	// Try to create index on non-existent field (should fail)
	if err := coll.CreateIndex("nonexistent", invertParam, DefaultIndexOption()); err == nil {
		t.Error("Expected error for non-existent field")
	}
}

func TestCollectionDropIndex(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Drop index (should not error even if no index exists)
	if err := coll.DropIndex("embedding"); err != nil {
		t.Fatalf("DropIndex failed: %v", err)
	}
}

func TestCollectionOptimize(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Optimize
	if err := coll.Optimize(DefaultOptimizeOption()); err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}
}

func TestCollectionAddColumn(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Add column
	newField := NewFieldSchema("category", DataTypeString)
	if err := coll.AddColumn(newField, "", DefaultAddColumnOption()); err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	// Verify field was added
	found := false
	for _, f := range coll.schema.Fields {
		if f.Name == "category" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected field 'category' to be added")
	}

	// Try to add duplicate field (should fail)
	if err := coll.AddColumn(newField, "", DefaultAddColumnOption()); err == nil {
		t.Error("Expected error for duplicate field")
	}
}

func TestCollectionDropColumn(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddField(NewFieldSchema("id", DataTypeInt64))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Drop column
	if err := coll.DropColumn("id"); err != nil {
		t.Fatalf("DropColumn failed: %v", err)
	}

	// Verify field was removed
	for _, f := range coll.schema.Fields {
		if f.Name == "id" {
			t.Error("Expected field 'id' to be removed")
		}
	}

	// Drop non-existent column (should fail)
	if err := coll.DropColumn("nonexistent"); err == nil {
		t.Error("Expected error for non-existent field")
	}
}

func TestCollectionUpsert(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert doc
	doc := NewDocument("doc1").SetField("title", "Original")
	if err := coll.Insert(doc); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Upsert (update)
	doc = NewDocument("doc1").SetField("title", "Updated")
	if err := coll.Upsert(doc); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Verify update
	retDoc, err := coll.Get("doc1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	title, _ := retDoc.GetField("title")
	if title != "Updated" {
		t.Errorf("Expected title 'Updated', got '%v'", title)
	}
}

func TestCollectionFetch(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert docs
	for i := 0; i < 3; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Fetch multiple
	results, err := coll.Fetch([]string{"doc0", "doc1", "doc2", "nonexistent"})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	if _, ok := results["doc0"]; !ok {
		t.Error("Expected doc0 in results")
	}

	// nonexistent should not be in results
	if _, ok := results["nonexistent"]; ok {
		t.Error("Did not expect nonexistent in results")
	}
}

func TestQueryExecutor(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 3))

	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert docs
	docs := []*Document{
		NewDocument("doc1").SetVector("embedding", []float32{1.0, 0.0, 0.0}),
		NewDocument("doc2").SetVector("embedding", []float32{0.0, 1.0, 0.0}),
	}
	for _, doc := range docs {
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Create query executor
	executor := NewQueryExecutor(schema)

	// Execute query
	ctx := &QueryContext{
		Query:         NewVectorQueryByVector("embedding", []float32{1.0, 0.0, 0.0}).WithTopK(2),
		TopK:          2,
		IncludeVector: true,
	}

	results, err := executor.Execute(ctx, coll)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected query results")
	}
}

func TestListIDs(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("test")
	collPath := filepath.Join(tmpDir, "test")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert docs
	for i := 0; i < 3; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		if err := coll.Insert(doc); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// List IDs
	ids, err := coll.ListIDs()
	if err != nil {
		t.Fatalf("ListIDs failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("Expected 3 IDs, got %d", len(ids))
	}
}
