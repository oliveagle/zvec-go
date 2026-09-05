package server

import "github.com/oliveagle/zvec-go"

// CollectionInfo is the JSON representation of a collection's metadata and
// statistics.
type CollectionInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	DocCount    int64                  `json:"doc_count"`
	SizeBytes   int64                  `json:"size_bytes"`
	Schema      *zvec.CollectionSchema `json:"schema,omitempty"`
}

// info builds a CollectionInfo for the named open collection.
func (m *CollectionManager) info(name string, c *zvec.Collection) *CollectionInfo {
	out := &CollectionInfo{
		Name:     name,
		Schema:   c.Schema(),
	}
	if s := c.Schema(); s != nil {
		out.Description = s.Description
	}
	if st, err := c.Stats(); err == nil && st != nil {
		out.DocCount = st.DocCount
		out.SizeBytes = st.SizeBytes
	}
	return out
}

// fieldSpec describes a scalar field in a collection creation request.
type fieldSpec struct {
	Name     string        `json:"name"`
	DataType zvec.DataType `json:"data_type"`
	Nullable bool          `json:"nullable,omitempty"`
}

// vectorFieldSpec describes a vector field in a collection creation request.
type vectorFieldSpec struct {
	Name       string          `json:"name"`
	DataType   zvec.DataType   `json:"data_type"`
	Dimension  int             `json:"dimension"`
	MetricType zvec.MetricType `json:"metric_type,omitempty"`
}

// createCollectionRequest is the body of POST /api/v1/collections.
type createCollectionRequest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Fields       []fieldSpec       `json:"fields"`
	VectorFields []vectorFieldSpec `json:"vector_fields"`
}

// documentSpec is a single document (used for upsert/get/list).
type documentSpec struct {
	ID       string                 `json:"id"`
	Fields   map[string]interface{} `json:"fields,omitempty"`
	Vectors  map[string][]float32   `json:"vectors,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// upsertRequest is the body of POST .../documents. Provide either a single
// "document" or a batch "documents".
type upsertRequest struct {
	Document  *documentSpec  `json:"document,omitempty"`
	Documents []documentSpec `json:"documents,omitempty"`
}

// queryRequest is the body of POST .../query.
type queryRequest struct {
	Field         string    `json:"field"`
	Vector        []float32 `json:"vector"`
	TopK          int       `json:"top_k"`
	OutputFields  []string  `json:"output_fields,omitempty"`
	IncludeVector bool      `json:"include_vector,omitempty"`
}

// deleteDocumentsRequest is the body for bulk delete (optional; a single id can
// also be supplied via the URL path).
type deleteDocumentsRequest struct {
	IDs []string `json:"ids"`
}

// queryResponse wraps vector search results.
type queryResponse struct {
	Results []*zvec.QueryResult `json:"results"`
	TookMs  int64              `json:"took_ms"`
}

// listDocumentsResponse wraps a paginated document listing.
type listDocumentsResponse struct {
	Total     int64           `json:"total"`
	Documents []*zvec.Document `json:"documents"`
}

// listCollectionsResponse wraps the collection list.
type listCollectionsResponse struct {
	Count       int               `json:"count"`
	Collections []*CollectionInfo `json:"collections"`
}

// upsertResponse reports how many documents were written.
type upsertResponse struct {
	Written int `json:"written"`
}

// healthResponse is returned by GET /healthz.
type healthResponse struct {
	Status      string `json:"status"`
	Version     string `json:"version"`
	Collections int    `json:"collections"`
	AuthEnabled bool   `json:"auth_enabled"`
}
