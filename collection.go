package zvec

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if ok {
		return doc, nil
	}
	// Not in memory; load from disk (best-effort). We intentionally do not
	// cache here while holding a read lock to avoid a write under RLock.
	doc, err := c.readDocument(id)
	if err != nil {
		return nil, fmt.Errorf("document not found: %s", id)
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
		score := similarity(queryVec, vec, c.metricFor(query.FieldName))
		results = append(results, &SearchResult{
			ID:       id,
			Score:    score,
			Document: doc,
		})
	}

	// Sort by score (descending), then limit to TopK.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
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
	sort.Strings(ids)
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

// loadDocs populates the in-memory document map from disk so that queries,
// listings, and statistics reflect persisted data across process restarts.
// Callers must not already hold c.mu.
func (c *Collection) loadDocs() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.docs == nil {
		c.docs = make(map[string]*Document)
	}

	docsDir := filepath.Join(c.path, "docs")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(docsDir, name))
		if err != nil {
			continue
		}
		var doc Document
		if err := json.Unmarshal(data, &doc); err != nil {
			continue
		}
		if doc.ID == "" {
			doc.ID = strings.TrimSuffix(name, ".json")
		}
		c.docs[doc.ID] = &doc
	}
	return nil
}

// ListDocs returns documents in the collection in stable (ID) order, applying
// an optional offset and limit. A limit of 0 means "no limit".
func (c *Collection) ListDocs(offset, limit int) ([]*Document, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	ids := make([]string, 0, len(c.docs))
	for id := range c.docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if offset < 0 {
		offset = 0
	}
	if offset > len(ids) {
		offset = len(ids)
	}
	ids = ids[offset:]
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}

	out := make([]*Document, 0, len(ids))
	for _, id := range ids {
		out = append(out, c.docs[id])
	}
	return out, nil
}

// cosineSimilarity computes cosine similarity between two vectors in [-1,1].
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

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// metricFor returns the metric type configured for a vector field. The caller
// must already hold c.mu (at least RLock). It defaults to COSINE.
func (c *Collection) metricFor(fieldName string) MetricType {
	if c.schema != nil {
		for _, v := range c.schema.VectorFields {
			if v.Name == fieldName && v.MetricType != "" {
				return v.MetricType
			}
		}
	}
	return MetricTypeCOSINE
}

// similarity returns a score where HIGHER means more similar, for the given
// metric. It is used to order search/query results (descending).
func similarity(a, b []float32, metric MetricType) float64 {
	if len(a) != len(b) {
		return math.Inf(-1)
	}
	switch metric {
	case MetricTypeIP:
		var dot float64
		for i := range a {
			dot += float64(a[i] * b[i])
		}
		return dot
	case MetricTypeL2:
		var d2 float64
		for i := range a {
			d := float64(a[i] - b[i])
			d2 += d * d
		}
		return -d2
	default: // MetricTypeCOSINE
		return cosineSimilarity(a, b)
	}
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

// ========== Collection DDL Methods ==========

// Destroy permanently deletes the collection from disk.
// This operation is irreversible - all data will be lost.
func (c *Collection) Destroy() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true

	// Remove the collection directory. Idempotent: destroying an already
	// closed collection simply (re)removes the data.
	return os.RemoveAll(c.path)
}

// Flush forces all pending writes to disk.
// Ensures durability of recent inserts/updates.
func (c *Collection) Flush() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// Force sync all documents to disk
	for _, doc := range c.docs {
		if err := c.writeDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

// Stats returns runtime statistics about the collection.
func (c *Collection) Stats() (*CollectionStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	// Calculate on-disk size by summing the sizes of all document files.
	var sizeBytes int64
	docsDir := filepath.Join(c.path, "docs")
	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if fi, err := e.Info(); err == nil {
				sizeBytes += fi.Size()
			}
		}
	}

	return &CollectionStats{
		DocCount:  int64(len(c.docs)),
		SizeBytes: sizeBytes,
	}, nil
}

// ========== Index DDL Methods ==========

// CreateIndex creates an index on a field.
// Vector index types (HNSW, IVF, FLAT) can only be applied to vector fields.
// Inverted index (InvertIndexParam) is for scalar fields.
func (c *Collection) CreateIndex(fieldName string, indexParam interface{}, option *IndexOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// Validate field exists
	var found bool
	for _, f := range c.schema.Fields {
		if f.Name == fieldName {
			found = true
			break
		}
	}
	for _, v := range c.schema.VectorFields {
		if v.Name == fieldName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("field not found: %s", fieldName)
	}

	// TODO: Actual index creation
	// For now, this is a placeholder
	_ = option
	return nil
}

// DropIndex removes the index from a field.
func (c *Collection) DropIndex(fieldName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// TODO: Actual index drop
	return nil
}

// Optimize optimizes the collection (e.g., merge segments, rebuild index).
func (c *Collection) Optimize(option *OptimizeOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// TODO: Actual optimization
	_ = option
	return nil
}

// ========== Column DDL Methods ==========

// AddColumn adds a new column to the collection.
// The column is populated using the provided expression.
func (c *Collection) AddColumn(fieldSchema *FieldSchema, expression string, option *AddColumnOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	if fieldSchema == nil {
		return fmt.Errorf("field schema cannot be nil")
	}

	// Check for duplicate field name
	for _, f := range c.schema.Fields {
		if f.Name == fieldSchema.Name {
			return fmt.Errorf("field already exists: %s", fieldSchema.Name)
		}
	}
	for _, v := range c.schema.VectorFields {
		if v.Name == fieldSchema.Name {
			return fmt.Errorf("field already exists: %s", fieldSchema.Name)
		}
	}

	// Add field to schema
	c.schema.AddField(fieldSchema)

	// TODO: Apply expression to existing documents
	_ = expression
	_ = option
	return nil
}

// DropColumn removes a column from the collection.
func (c *Collection) DropColumn(fieldName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// Remove field from schema
	for i, f := range c.schema.Fields {
		if f.Name == fieldName {
			c.schema.Fields = append(c.schema.Fields[:i], c.schema.Fields[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("field not found: %s", fieldName)
}

// AlterColumn renames a column or updates its schema.
// This operation only supports scalar numeric columns.
func (c *Collection) AlterColumn(oldName string, newName string, fieldSchema *FieldSchema, option *AlterColumnOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// Find and update field
	for _, f := range c.schema.Fields {
		if f.Name == oldName {
			if newName != "" {
				f.Name = newName
			}
			if fieldSchema != nil {
				f.DataType = fieldSchema.DataType
				f.Nullable = fieldSchema.Nullable
				f.IndexParam = fieldSchema.IndexParam
			}
			return nil
		}
	}

	return fmt.Errorf("field not found: %s", oldName)
}

// ========== DML Methods ==========

// Upsert inserts new documents or updates existing ones by ID.
func (c *Collection) Upsert(doc *Document) error {
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

	c.docs[doc.ID] = doc
	return c.writeDocument(doc)
}

// UpsertBatch upserts multiple documents.
func (c *Collection) UpsertBatch(docs []*Document) (int, error) {
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

// DeleteByFilter deletes documents matching a filter expression.
func (c *Collection) DeleteByFilter(filter string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("collection is closed")
	}

	// TODO: Implement filter parsing and evaluation
	// For now, this is a placeholder
	_ = filter
	return fmt.Errorf("DeleteByFilter not yet implemented")
}

// ========== DQL Methods ==========

// Fetch retrieves documents by ID.
// Returns a map from ID to document. Missing IDs are omitted.
func (c *Collection) Fetch(ids []string) (map[string]*Document, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("collection is closed")
	}

	results := make(map[string]*Document)
	for _, id := range ids {
		doc, err := c.Get(id)
		if err == nil && doc != nil {
			results[id] = doc
		}
	}
	return results, nil
}

// QueryResult represents a single query result.
type QueryResult struct {
	ID     string            `json:"id"`
	Score  float64           `json:"score"`
	Fields map[string]interface{} `json:"fields,omitempty"`
	Vector map[string][]float32 `json:"vectors,omitempty"`
}

// Query performs a vector similarity search with optional filtering and re-ranking.
func (c *Collection) Query(query *VectorQuery, topk int, filter string, includeVector bool, outputFields []string) ([]*QueryResult, error) {
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
		doc, ok := c.docs[query.ID]
		if !ok {
			var err error
			doc, err = c.readDocument(query.ID)
			if err != nil {
				return nil, fmt.Errorf("document not found: %s", query.ID)
			}
		}
		vec, ok := doc.Vectors[query.FieldName]
		if !ok {
			return nil, fmt.Errorf("vector field not found: %s", query.FieldName)
		}
		queryVec = vec
	} else {
		queryVec = query.Vector
	}

	// Perform search
	results := make([]*QueryResult, 0)
	for id, doc := range c.docs {
		vec, ok := doc.Vectors[query.FieldName]
		if !ok {
			continue
		}
		score := similarity(queryVec, vec, c.metricFor(query.FieldName))

		// Build result with selected fields
		result := &QueryResult{
			ID:     id,
			Score:  score,
			Fields: make(map[string]interface{}),
			Vector: make(map[string][]float32),
		}

		// Include selected fields
		if outputFields == nil {
			// Include all fields
			for k, v := range doc.Fields {
				result.Fields[k] = v
			}
		} else {
			for _, k := range outputFields {
				if v, ok := doc.Fields[k]; ok {
					result.Fields[k] = v
				}
			}
		}

		// Include vector if requested
		if includeVector {
			for k, v := range doc.Vectors {
				result.Vector[k] = v
			}
		}

		results = append(results, result)
	}

	// Sort by score (descending), then limit to topk.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if topk > 0 && len(results) > topk {
		results = results[:topk]
	}

	return results, nil
}

// ReRanker interface for re-ranking query results.
type ReRanker interface {
	Rerank(query string, results []*QueryResult) []*QueryResult
}

// QueryContext holds the context for a query execution.
type QueryContext struct {
	Query         *VectorQuery
	TopK          int
	Filter        string
	IncludeVector bool
	OutputFields  []string
	ReRanker      ReRanker
}

// QueryExecutor executes queries.
type QueryExecutor struct {
	schema *CollectionSchema
}

// NewQueryExecutor creates a new query executor.
func NewQueryExecutor(schema *CollectionSchema) *QueryExecutor {
	return &QueryExecutor{
		schema: schema,
	}
}

// Execute executes a query on the collection.
func (e *QueryExecutor) Execute(ctx *QueryContext, c *Collection) ([]*QueryResult, error) {
	results, err := c.Query(
		ctx.Query,
		ctx.TopK,
		ctx.Filter,
		ctx.IncludeVector,
		ctx.OutputFields,
	)
	if err != nil {
		return nil, err
	}

	// Apply re-ranking if provided
	if ctx.ReRanker != nil {
		results = ctx.ReRanker.Rerank("", results)
	}

	return results, nil
}
