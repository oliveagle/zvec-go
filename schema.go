package zvec

import (
	"encoding/json"
	"fmt"
)

// CollectionSchema defines the structure of a collection.
type CollectionSchema struct {
	Name          string           `json:"name"`
	Fields        []*FieldSchema   `json:"fields,omitempty"`
	VectorFields  []*VectorSchema  `json:"vector_fields,omitempty"`
	Description   string           `json:"description,omitempty"`
}

// NewCollectionSchema creates a new CollectionSchema.
func NewCollectionSchema(name string) *CollectionSchema {
	return &CollectionSchema{
		Name:         name,
		Fields:       make([]*FieldSchema, 0),
		VectorFields: make([]*VectorSchema, 0),
	}
}

// AddField adds a scalar field to the schema.
func (s *CollectionSchema) AddField(field *FieldSchema) *CollectionSchema {
	s.Fields = append(s.Fields, field)
	return s
}

// AddVectorField adds a vector field to the schema.
func (s *CollectionSchema) AddVectorField(field *VectorSchema) *CollectionSchema {
	s.VectorFields = append(s.VectorFields, field)
	return s
}

// Validate validates the schema.
func (s *CollectionSchema) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	// Check for duplicate field names
	fieldNames := make(map[string]bool)
	for _, f := range s.Fields {
		if fieldNames[f.Name] {
			return fmt.Errorf("duplicate field name: %s", f.Name)
		}
		fieldNames[f.Name] = true
	}
	for _, f := range s.VectorFields {
		if fieldNames[f.Name] {
			return fmt.Errorf("duplicate field name: %s", f.Name)
		}
		fieldNames[f.Name] = true
	}

	return nil
}

// FieldSchema represents a scalar field in a collection schema.
type FieldSchema struct {
	Name       string           `json:"name"`
	DataType   DataType         `json:"data_type"`
	Nullable   bool             `json:"nullable"`
	IndexParam *InvertIndexParam `json:"index_param,omitempty"`
}

// NewFieldSchema creates a new FieldSchema.
func NewFieldSchema(name string, dataType DataType) *FieldSchema {
	return &FieldSchema{
		Name:     name,
		DataType: dataType,
		Nullable: false,
	}
}

// WithNullable sets whether the field can be null.
func (f *FieldSchema) WithNullable(nullable bool) *FieldSchema {
	f.Nullable = nullable
	return f
}

// WithIndexParam sets the index parameters.
func (f *FieldSchema) WithIndexParam(param *InvertIndexParam) *FieldSchema {
	f.IndexParam = param
	return f
}

// Validate validates the field schema.
func (f *FieldSchema) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("field name cannot be empty")
	}
	if !f.DataType.IsScalar() {
		return fmt.Errorf("field %s must be a scalar type, got %s", f.Name, f.DataType)
	}
	return nil
}

// VectorSchema represents a vector field in a collection schema.
type VectorSchema struct {
	Name       string                              `json:"name"`
	DataType   DataType                            `json:"data_type"`
	Dimension  int                                 `json:"dimension"`
	MetricType MetricType                          `json:"metric_type,omitempty"`
	IndexParam interface{}                         `json:"index_param,omitempty"` // HnswIndexParam, IVFIndexParam, or FlatIndexParam
}

// NewVectorSchema creates a new VectorSchema.
func NewVectorSchema(name string, dataType DataType, dimension int) *VectorSchema {
	return &VectorSchema{
		Name:       name,
		DataType:   dataType,
		Dimension:  dimension,
		MetricType: MetricTypeL2,
	}
}

// WithMetricType sets the metric type for similarity search.
func (v *VectorSchema) WithMetricType(metric MetricType) *VectorSchema {
	v.MetricType = metric
	return v
}

// WithIndexParam sets the index parameters.
func (v *VectorSchema) WithIndexParam(param interface{}) *VectorSchema {
	v.IndexParam = param
	return v
}

// Validate validates the vector schema.
func (v *VectorSchema) Validate() error {
	if v.Name == "" {
		return fmt.Errorf("vector field name cannot be empty")
	}
	if !v.DataType.IsVector() {
		return fmt.Errorf("field %s must be a vector type, got %s", v.Name, v.DataType)
	}
	if v.Dimension <= 0 && v.DataType != DataTypeSparseVectorFP16 && v.DataType != DataTypeSparseVectorFP32 {
		return fmt.Errorf("vector dimension must be > 0 for dense vectors")
	}
	return nil
}

// InvertIndexParam contains parameters for inverted index.
type InvertIndexParam struct {
	EnableRangeOptimization bool `json:"enable_range_optimization,omitempty"`
}

// NewInvertIndexParam creates a new InvertIndexParam.
func NewInvertIndexParam() *InvertIndexParam {
	return &InvertIndexParam{
		EnableRangeOptimization: false,
	}
}

// WithEnableRangeOptimization enables range optimization.
func (i *InvertIndexParam) WithEnableRangeOptimization(enable bool) *InvertIndexParam {
	i.EnableRangeOptimization = enable
	return i
}

// HnswIndexParam contains parameters for HNSW index.
type HnswIndexParam struct {
	M              int `json:"m,omitempty"`
	EfConstruction int `json:"ef_construction,omitempty"`
	EfSearch       int `json:"ef_search,omitempty"`
}

// NewHnswIndexParam creates a new HnswIndexParam with default values.
func NewHnswIndexParam() *HnswIndexParam {
	return &HnswIndexParam{
		M:              16,
		EfConstruction: 200,
		EfSearch:       128,
	}
}

// WithM sets the M parameter (max number of connections per layer).
func (h *HnswIndexParam) WithM(m int) *HnswIndexParam {
	h.M = m
	return h
}

// WithEfConstruction sets the efConstruction parameter.
func (h *HnswIndexParam) WithEfConstruction(ef int) *HnswIndexParam {
	h.EfConstruction = ef
	return h
}

// WithEfSearch sets the efSearch parameter.
func (h *HnswIndexParam) WithEfSearch(ef int) *HnswIndexParam {
	h.EfSearch = ef
	return h
}

// IVFIndexParam contains parameters for IVF index.
type IVFIndexParam struct {
	NList   int `json:"nlist,omitempty"`
	NProbe  int `json:"nprobe,omitempty"`
}

// NewIVFIndexParam creates a new IVFIndexParam with default values.
func NewIVFIndexParam() *IVFIndexParam {
	return &IVFIndexParam{
		NList:  1024,
		NProbe: 8,
	}
}

// WithNList sets the number of clusters.
func (i *IVFIndexParam) WithNList(n int) *IVFIndexParam {
	i.NList = n
	return i
}

// WithNProbe sets the number of probes to use at search time.
func (i *IVFIndexParam) WithNProbe(n int) *IVFIndexParam {
	i.NProbe = n
	return i
}

// FlatIndexParam contains parameters for flat (brute-force) index.
type FlatIndexParam struct {
	MetricType MetricType `json:"metric_type,omitempty"`
}

// NewFlatIndexParam creates a new FlatIndexParam.
func NewFlatIndexParam() *FlatIndexParam {
	return &FlatIndexParam{
		MetricType: MetricTypeL2,
	}
}

// WithMetricType sets the metric type.
func (f *FlatIndexParam) WithMetricType(metric MetricType) *FlatIndexParam {
	f.MetricType = metric
	return f
}

// CollectionOption contains options for opening a collection.
type CollectionOption struct {
	ReadOnly        bool `json:"read_only,omitempty"`
	CreateIfMissing bool `json:"create_if_missing,omitempty"`
	ErrorIfExists   bool `json:"error_if_exists,omitempty"`
}

// DefaultCollectionOption returns a default CollectionOption.
func DefaultCollectionOption() *CollectionOption {
	return &CollectionOption{
		ReadOnly:        false,
		CreateIfMissing: true,
		ErrorIfExists:   false,
	}
}

// WithReadOnly sets whether to open the collection in read-only mode.
func (o *CollectionOption) WithReadOnly(readonly bool) *CollectionOption {
	o.ReadOnly = readonly
	return o
}

// WithCreateIfMissing sets whether to create the collection if it doesn't exist.
func (o *CollectionOption) WithCreateIfMissing(create bool) *CollectionOption {
	o.CreateIfMissing = create
	return o
}

// WithErrorIfExists sets whether to error if the collection already exists.
func (o *CollectionOption) WithErrorIfExists(errorIfExists bool) *CollectionOption {
	o.ErrorIfExists = errorIfExists
	return o
}

// HnswQueryParam contains HNSW-specific query parameters.
type HnswQueryParam struct {
	Ef int `json:"ef,omitempty"`
}

// NewHnswQueryParam creates a new HnswQueryParam.
func NewHnswQueryParam() *HnswQueryParam {
	return &HnswQueryParam{
		Ef: 128,
	}
}

// WithEf sets the ef parameter for HNSW search.
func (h *HnswQueryParam) WithEf(ef int) *HnswQueryParam {
	h.Ef = ef
	return h
}

// IVFQueryParam contains IVF-specific query parameters.
type IVFQueryParam struct {
	NProbe int `json:"nprobe,omitempty"`
}

// NewIVFQueryParam creates a new IVFQueryParam.
func NewIVFQueryParam() *IVFQueryParam {
	return &IVFQueryParam{
		NProbe: 8,
	}
}

// WithNProbe sets the nprobe parameter for IVF search.
func (i *IVFQueryParam) WithNProbe(n int) *IVFQueryParam {
	i.NProbe = n
	return i
}

// VectorQuery represents a vector search query.
type VectorQuery struct {
	FieldName string      `json:"field_name"`
	ID        string      `json:"id,omitempty"`
	Vector    []float32   `json:"vector,omitempty"`
	Param     interface{} `json:"param,omitempty"` // HnswQueryParam or IVFQueryParam
	TopK      int         `json:"top_k,omitempty"`
}

// NewVectorQueryByID creates a VectorQuery using a document ID.
func NewVectorQueryByID(fieldName, id string) *VectorQuery {
	return &VectorQuery{
		FieldName: fieldName,
		ID:        id,
		TopK:      10,
	}
}

// NewVectorQueryByVector creates a VectorQuery using an explicit vector.
func NewVectorQueryByVector(fieldName string, vector []float32) *VectorQuery {
	return &VectorQuery{
		FieldName: fieldName,
		Vector:    vector,
		TopK:      10,
	}
}

// WithTopK sets the number of results to return.
func (q *VectorQuery) WithTopK(k int) *VectorQuery {
	q.TopK = k
	return q
}

// WithParam sets the query parameters.
func (q *VectorQuery) WithParam(param interface{}) *VectorQuery {
	q.Param = param
	return q
}

// HasID checks if the query uses a document ID.
func (q *VectorQuery) HasID() bool {
	return q.ID != ""
}

// HasVector checks if the query uses an explicit vector.
func (q *VectorQuery) HasVector() bool {
	return len(q.Vector) > 0
}

// Validate validates the vector query.
func (q *VectorQuery) Validate() error {
	if q.FieldName == "" {
		return fmt.Errorf("field name cannot be empty")
	}
	if q.HasID() && q.HasVector() {
		return fmt.Errorf("cannot provide both id and vector")
	}
	if !q.HasID() && !q.HasVector() {
		return fmt.Errorf("must provide either id or vector")
	}
	return nil
}

// String returns a string representation of the schema.
func (s *CollectionSchema) String() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

// String returns a string representation of the field.
func (f *FieldSchema) String() string {
	data, _ := json.MarshalIndent(f, "", "  ")
	return string(data)
}

// String returns a string representation of the vector field.
func (v *VectorSchema) String() string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

// IndexOption contains options for creating an index.
type IndexOption struct {
	Async bool `json:"async,omitempty"`
}

// DefaultIndexOption returns a default IndexOption.
func DefaultIndexOption() *IndexOption {
	return &IndexOption{
		Async: false,
	}
}

// WithAsync sets whether to create the index asynchronously.
func (o *IndexOption) WithAsync(async bool) *IndexOption {
	o.Async = async
	return o
}

// OptimizeOption contains options for optimizing a collection.
type OptimizeOption struct {
	Full bool `json:"full,omitempty"`
}

// DefaultOptimizeOption returns a default OptimizeOption.
func DefaultOptimizeOption() *OptimizeOption {
	return &OptimizeOption{
		Full: false,
	}
}

// WithFull sets whether to perform a full optimization.
func (o *OptimizeOption) WithFull(full bool) *OptimizeOption {
	o.Full = full
	return o
}

// AddColumnOption contains options for adding a column.
type AddColumnOption struct {
	SkipBackfill bool `json:"skip_backfill,omitempty"`
}

// DefaultAddColumnOption returns a default AddColumnOption.
func DefaultAddColumnOption() *AddColumnOption {
	return &AddColumnOption{
		SkipBackfill: false,
	}
}

// WithSkipBackfill sets whether to skip backfilling existing documents.
func (o *AddColumnOption) WithSkipBackfill(skip bool) *AddColumnOption {
	o.SkipBackfill = skip
	return o
}

// AlterColumnOption contains options for altering a column.
type AlterColumnOption struct {
	SkipReindex bool `json:"skip_reindex,omitempty"`
}

// DefaultAlterColumnOption returns a default AlterColumnOption.
func DefaultAlterColumnOption() *AlterColumnOption {
	return &AlterColumnOption{
		SkipReindex: false,
	}
}

// WithSkipReindex sets whether to skip reindexing.
func (o *AlterColumnOption) WithSkipReindex(skip bool) *AlterColumnOption {
	o.SkipReindex = skip
	return o
}

// CollectionStats represents runtime statistics about a collection.
type CollectionStats struct {
	DocCount   int64  `json:"doc_count"`
	SizeBytes  int64  `json:"size_bytes"`
	MemoryBytes int64 `json:"memory_bytes"`
	IndexSize  int64  `json:"index_size"`
}

// String returns a string representation of the stats.
func (s *CollectionStats) String() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}
