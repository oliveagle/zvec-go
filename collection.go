package zvec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Collection represents an open zvec collection.
type Collection struct {
	mu     sync.RWMutex
	path   string
	schema *CollectionSchema
	option *CollectionOption
	docs   map[string]*Document // In-memory storage for demonstration
	closed bool
}

// Document represents a document in the collection.
type Document struct {
	ID       string                 `json:"id"`
	Fields   map[string]interface{} `json:"fields"`
	Vectors  map[string][]float32   `json:"vectors"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewDocument creates a new document.
func NewDocument(id string) *Document {
	return &Document{
		ID:       id,
		Fields:   make(map[string]interface{}),
		Vectors:  make(map[string][]float32),
		Metadata: make(map[string]interface{}),
	}
}

// SetField sets a field value.
func (d *Document) SetField(name string, value interface{}) *Document {
	d.Fields[name] = value
	return d
}

// SetVector sets a vector field.
func (d *Document) SetVector(name string, vector []float32) *Document {
	d.Vectors[name] = vector
	return d
}

// SetMetadata sets metadata.
func (d *Document) SetMetadata(key string, value interface{}) *Document {
	d.Metadata[key] = value
	return d
}

// GetField gets a field value.
func (d *Document) GetField(name string) (interface{}, bool) {
	val, ok := d.Fields[name]
	return val, ok
}

// GetVector gets a vector field.
func (d *Document) GetVector(name string) ([]float32, bool) {
	vec, ok := d.Vectors[name]
	return vec, ok
}

// Path returns the collection path.
func (c *Collection) Path() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.path
}

// Schema returns the collection schema.
func (c *Collection) Schema() *CollectionSchema {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.schema
}

// Closed returns whether the collection is closed.
func (c *Collection) Closed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Close closes the collection.
func (c *Collection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// TODO: Save any pending changes to disk
	c.closed = true
	return nil
}

// Insert inserts a document into the collection.
func (c *Collection) Insert(doc *Document) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}

	// TODO: Validate against schema
	c.docs[doc.ID] = doc

	// Write to disk for persistence
	return c.writeDocument(doc)
}

// InsertBatch inserts multiple documents.
func (c *Collection) InsertBatch(docs []*Document) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, fmt.Errorf("collection is closed")
	}

	count := 0
	for _, doc := range docs {
		if doc == nil || doc.ID == "" {
			continue
		}
		c.docs[doc.ID] = doc
		if err := c.writeDocument(doc); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// Get retrieves a document by ID.
func (c *Collection) Get(id string) (*Document, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	doc, ok := c.docs[id]
	if !ok {
		// Try to load from disk
		var err error
		doc, err = c.readDocument(id)
		if err != nil {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		c.docs[id] = doc
	}
	return doc, nil
}

// Delete deletes a document by ID.
func (c *Collection) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	delete(c.docs, id)
	return c.deleteDocument(id)
}

// Update updates a document.
func (c *Collection) Update(doc *Document) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	if doc == nil || doc.ID == "" {
		return fmt.Errorf("invalid document")
	}

	c.docs[doc.ID] = doc
	return c.writeDocument(doc)
}

// SearchResult represents a single search result.
type SearchResult struct {
	ID       string  `json:"id"`
	Score    float64 `json:"score"`
	Document *Document `json:"document,omitempty"`
}

// Search performs a vector similarity search.
func (c *Collection) Search(query *VectorQuery) ([]*SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	if err := query.Validate(); err != nil {
		return nil, err
	}

	// Get query vector
	var queryVec []float32
	if query.HasID() {
		doc, err := c.readDocument(query.ID)
		if err != nil {
			return nil, fmt.Errorf("document not found: %s", query.ID)
		}
		vec, ok := doc.Vectors[query.FieldName]
		if !ok {
			return nil, fmt.Errorf("vector field not found: %s", query.FieldName)
		}
		queryVec = vec
	} else {
		queryVec = query.Vector
	}

	// Perform search (simple brute-force for demonstration)
	results := make([]*SearchResult, 0)
	for id, doc := range c.docs {
		vec, ok := doc.Vectors[query.FieldName]
		if !ok {
			continue
		}
		score := cosineSimilarity(queryVec, vec)
		results = append(results, &SearchResult{
			ID:       id,
			Score:    score,
			Document: doc,
		})
	}

	// Sort and limit results
	// TODO: Implement proper sorting
	if query.TopK > 0 && len(results) > query.TopK {
		results = results[:query.TopK]
	}

	return results, nil
}

// Count returns the number of documents in the collection.
func (c *Collection) Count() (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("collection is closed")
	}

	return int64(len(c.docs)), nil
}

// ListIDs returns all document IDs in the collection.
func (c *Collection) ListIDs() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	ids := make([]string, 0, len(c.docs))
	for id := range c.docs {
		ids = append(ids, id)
	}
	return ids, nil
}

// Internal helper methods

func (c *Collection) docPath(id string) string {
	return filepath.Join(c.path, "docs", id+".json")
}

func (c *Collection) writeDocument(doc *Document) error {
	docsDir := filepath.Join(c.path, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.docPath(doc.ID), data, 0644)
}

func (c *Collection) readDocument(id string) (*Document, error) {
	data, err := os.ReadFile(c.docPath(id))
	if err != nil {
		return nil, err
	}

	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (c *Collection) deleteDocument(id string) error {
	return os.Remove(c.docPath(id))
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

// String returns a string representation of the document.
func (d *Document) String() string {
	data, _ := json.MarshalIndent(d, "", "  ")
	return string(data)
}

// String returns a string representation of the search result.
func (r *SearchResult) String() string {
	return fmt.Sprintf("ID: %s, Score: %.6f", r.ID, r.Score)
}
