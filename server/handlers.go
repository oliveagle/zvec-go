package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/oliveagle/zvec-go"
)

const maxBodyBytes = 10 << 20 // 10 MiB

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeBody decodes a JSON request body of bounded size into dst.
func decodeBody(r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

// getCollection resolves the {name} path parameter to an open collection.
func (s *Server) getCollection(w http.ResponseWriter, r *http.Request) (*zvec.Collection, bool) {
	name := r.PathValue("name")
	if !ValidateCollectionName(name) {
		writeError(w, http.StatusBadRequest, "invalid collection name")
		return nil, false
	}
	c, ok := s.mgr.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "collection not found: "+name)
		return nil, false
	}
	return c, true
}

// requireWritable rejects mutations for readonly users.
func requireWritable(w http.ResponseWriter, r *http.Request) bool {
	if isReadonly(r) {
		writeError(w, http.StatusForbidden, "readonly user cannot modify data")
		return false
	}
	return true
}

// handleHealth reports service health. It is not authenticated.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:      "ok",
		Version:     ServiceVersion,
		Collections: s.mgr.Count(),
		AuthEnabled: s.auth != nil,
	})
}

// handleListCollections lists all collections.
func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	collections := s.mgr.List()
	writeJSON(w, http.StatusOK, listCollectionsResponse{
		Count:       len(collections),
		Collections: collections,
	})
}

// handleCreateCollection creates a new collection or table.
func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	if !requireWritable(w, r) {
		return
	}
	var req createCollectionRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Name = nameOnly(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !ValidateCollectionName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid collection name (must match [A-Za-z0-9][A-Za-z0-9_-]{0,63})")
		return
	}

	schema := zvec.NewCollectionSchema(req.Name)
	schema.Description = req.Description
	for _, f := range req.Fields {
		if f.Name == "" || !f.DataType.IsScalar() {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("field %q must be a scalar data type", f.Name))
			return
		}
		schema.AddField(zvec.NewFieldSchema(f.Name, f.DataType).WithNullable(f.Nullable))
	}
	for _, v := range req.VectorFields {
		if v.Name == "" || !v.DataType.IsVector() {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("vector field %q must be a vector data type", v.Name))
			return
		}
		vs := zvec.NewVectorSchema(v.Name, v.DataType, v.Dimension)
		if v.MetricType != "" {
			vs = vs.WithMetricType(v.MetricType)
		}
		schema.AddVectorField(vs)
	}
	if err := schema.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid schema: "+err.Error())
		return
	}

	coll, err := s.mgr.Create(req.Name, schema)
	if err != nil {
		if err == errCollectionExists {
			writeError(w, http.StatusConflict, "collection already exists: "+req.Name)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Echo the newly created collection.
	writeJSON(w, http.StatusCreated, s.mgr.info(req.Name, coll))
}

// handleGetCollection returns one collection's metadata/stats.
func (s *Server) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.mgr.info(r.PathValue("name"), c))
}

// handleDropCollection permanently deletes a collection.
func (s *Server) handleDropCollection(w http.ResponseWriter, r *http.Request) {
	if !requireWritable(w, r) {
		return
	}
	name := r.PathValue("name")
	if err := s.mgr.Drop(name); err != nil {
		if err == errCollectionNotFound {
			writeError(w, http.StatusNotFound, "collection not found: "+name)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListDocuments lists documents with optional offset/limit.
func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	docs, err := c.ListDocs(offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, _ := c.Count()
	writeJSON(w, http.StatusOK, listDocumentsResponse{Total: total, Documents: docs})
}

// handleUpsertDocuments inserts or updates one or more documents.
func (s *Server) handleUpsertDocuments(w http.ResponseWriter, r *http.Request) {
	if !requireWritable(w, r) {
		return
	}
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	var req upsertRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	docs := make([]*zvec.Document, 0, len(req.Documents)+1)
	if req.Document != nil {
		docs = append(docs, toDocument(req.Document))
	}
	for i := range req.Documents {
		d := toDocument(&req.Documents[i])
		if d.ID == "" {
			writeError(w, http.StatusBadRequest, "documents["+strconv.Itoa(i)+"] is missing id")
			return
		}
		docs = append(docs, d)
	}
	if req.Document != nil && req.Document.ID == "" {
		writeError(w, http.StatusBadRequest, "document is missing id")
		return
	}
	if len(docs) == 0 {
		writeError(w, http.StatusBadRequest, "provide either 'document' or 'documents'")
		return
	}

	n, err := c.UpsertBatch(docs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, upsertResponse{Written: n})
}

// handleGetDocument fetches a single document by id.
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	doc, err := c.Get(id)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// handleDeleteDocument deletes a single document by id.
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	if !requireWritable(w, r) {
		return
	}
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := c.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleQuery runs a vector similarity search.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	c, ok := s.getCollection(w, r)
	if !ok {
		return
	}
	var req queryRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Field == "" || len(req.Vector) == 0 {
		writeError(w, http.StatusBadRequest, "field and vector are required")
		return
	}
	topk := req.TopK
	if topk <= 0 {
		topk = 10
	}
	q := zvec.NewVectorQueryByVector(req.Field, req.Vector).WithTopK(topk)

	start := time.Now()
	results, err := c.Query(q, topk, "", req.IncludeVector, req.OutputFields)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if results == nil {
		results = []*zvec.QueryResult{}
	}
	writeJSON(w, http.StatusOK, queryResponse{
		Results: results,
		TookMs:  time.Since(start).Milliseconds(),
	})
}

func toDocument(d *documentSpec) *zvec.Document {
	doc := zvec.NewDocument(d.ID)
	if d.Fields != nil {
		doc.Fields = make(map[string]interface{}, len(d.Fields))
		for k, v := range d.Fields {
			doc.Fields[k] = v
		}
	}
	if d.Vectors != nil {
		doc.Vectors = make(map[string][]float32, len(d.Vectors))
		for k, v := range d.Vectors {
			doc.Vectors[k] = v
		}
	}
	if d.Metadata != nil {
		doc.Metadata = make(map[string]interface{}, len(d.Metadata))
		for k, v := range d.Metadata {
			doc.Metadata[k] = v
		}
	}
	return doc
}
