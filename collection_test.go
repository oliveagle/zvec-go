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
