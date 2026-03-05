package zvec

import (
	"testing"
)

func TestCollectionSchema(t *testing.T) {
	schema := NewCollectionSchema("test_collection")
	if schema.Name != "test_collection" {
		t.Errorf("Expected name 'test_collection', got '%s'", schema.Name)
	}

	// Add a scalar field
	field := NewFieldSchema("id", DataTypeInt64).WithNullable(false)
	schema.AddField(field)

	if len(schema.Fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(schema.Fields))
	}

	// Add a vector field
	vecField := NewVectorSchema("embedding", DataTypeVectorFP32, 128)
	schema.AddVectorField(vecField)

	if len(schema.VectorFields) != 1 {
		t.Errorf("Expected 1 vector field, got %d", len(schema.VectorFields))
	}

	// Validate schema
	if err := schema.Validate(); err != nil {
		t.Errorf("Schema validation failed: %v", err)
	}
}

func TestFieldSchema(t *testing.T) {
	field := NewFieldSchema("test_field", DataTypeString)
	if field.Name != "test_field" {
		t.Errorf("Expected name 'test_field', got '%s'", field.Name)
	}
	if field.DataType != DataTypeString {
		t.Errorf("Expected DataTypeString, got %v", field.DataType)
	}

	// Test fluent methods
	field = field.WithNullable(true).WithIndexParam(NewInvertIndexParam().WithEnableRangeOptimization(true))

	if !field.Nullable {
		t.Error("Expected Nullable to be true")
	}
	if field.IndexParam == nil {
		t.Error("Expected IndexParam to be set")
	}
	if !field.IndexParam.EnableRangeOptimization {
		t.Error("Expected EnableRangeOptimization to be true")
	}

	// Validate
	if err := field.Validate(); err != nil {
		t.Errorf("Field validation failed: %v", err)
	}
}

func TestVectorSchema(t *testing.T) {
	vec := NewVectorSchema("embedding", DataTypeVectorFP32, 128)
	if vec.Name != "embedding" {
		t.Errorf("Expected name 'embedding', got '%s'", vec.Name)
	}
	if vec.DataType != DataTypeVectorFP32 {
		t.Errorf("Expected DataTypeVectorFP32, got %v", vec.DataType)
	}
	if vec.Dimension != 128 {
		t.Errorf("Expected dimension 128, got %d", vec.Dimension)
	}

	// Test fluent methods
	vec = vec.WithMetricType(MetricTypeCOSINE).WithIndexParam(NewHnswIndexParam().WithM(32))

	if vec.MetricType != MetricTypeCOSINE {
		t.Errorf("Expected MetricTypeCOSINE, got %v", vec.MetricType)
	}
	if vec.IndexParam == nil {
		t.Error("Expected IndexParam to be set")
	}

	// Validate
	if err := vec.Validate(); err != nil {
		t.Errorf("Vector validation failed: %v", err)
	}
}

func TestVectorQuery(t *testing.T) {
	// Test query by ID
	q1 := NewVectorQueryByID("embedding", "doc1")
	if !q1.HasID() {
		t.Error("Expected HasID() to be true")
	}
	if q1.HasVector() {
		t.Error("Expected HasVector() to be false")
	}

	// Test query by vector
	vec := []float32{1.0, 2.0, 3.0}
	q2 := NewVectorQueryByVector("embedding", vec)
	if q2.HasID() {
		t.Error("Expected HasID() to be false")
	}
	if !q2.HasVector() {
		t.Error("Expected HasVector() to be true")
	}

	// Test fluent methods
	q2 = q2.WithTopK(20).WithParam(NewHnswQueryParam().WithEf(256))
	if q2.TopK != 20 {
		t.Errorf("Expected TopK 20, got %d", q2.TopK)
	}

	// Validate
	if err := q2.Validate(); err != nil {
		t.Errorf("Query validation failed: %v", err)
	}
}

func TestDataTypes(t *testing.T) {
	// Test scalar types
	scalarTypes := []DataType{
		DataTypeInt32, DataTypeInt64, DataTypeUInt32, DataTypeUInt64,
		DataTypeFloat, DataTypeDouble, DataTypeString, DataTypeBool,
	}
	for _, dt := range scalarTypes {
		if !dt.IsScalar() {
			t.Errorf("Expected %v to be scalar", dt)
		}
		if dt.IsVector() {
			t.Errorf("Expected %v to not be vector", dt)
		}
	}

	// Test vector types
	vectorTypes := []DataType{
		DataTypeVectorFP16, DataTypeVectorFP32, DataTypeVectorFP64, DataTypeVectorInt8,
	}
	for _, dt := range vectorTypes {
		if dt.IsScalar() {
			t.Errorf("Expected %v to not be scalar", dt)
		}
		if !dt.IsVector() {
			t.Errorf("Expected %v to be vector", dt)
		}
	}
}

func TestIndexParams(t *testing.T) {
	// Test HnswIndexParam
	hnsw := NewHnswIndexParam().WithM(32).WithEfConstruction(400).WithEfSearch(256)
	if hnsw.M != 32 {
		t.Errorf("Expected M=32, got %d", hnsw.M)
	}
	if hnsw.EfConstruction != 400 {
		t.Errorf("Expected EfConstruction=400, got %d", hnsw.EfConstruction)
	}
	if hnsw.EfSearch != 256 {
		t.Errorf("Expected EfSearch=256, got %d", hnsw.EfSearch)
	}

	// Test IVFIndexParam
	ivf := NewIVFIndexParam().WithNList(2048).WithNProbe(16)
	if ivf.NList != 2048 {
		t.Errorf("Expected NList=2048, got %d", ivf.NList)
	}
	if ivf.NProbe != 16 {
		t.Errorf("Expected NProbe=16, got %d", ivf.NProbe)
	}

	// Test FlatIndexParam
	flat := NewFlatIndexParam().WithMetricType(MetricTypeIP)
	if flat.MetricType != MetricTypeIP {
		t.Errorf("Expected MetricTypeIP, got %v", flat.MetricType)
	}

	// Test InvertIndexParam
	invert := NewInvertIndexParam().WithEnableRangeOptimization(true)
	if !invert.EnableRangeOptimization {
		t.Error("Expected EnableRangeOptimization=true")
	}

	// Test QueryParams
	hnswQ := NewHnswQueryParam().WithEf(512)
	if hnswQ.Ef != 512 {
		t.Errorf("Expected Ef=512, got %d", hnswQ.Ef)
	}

	ivfQ := NewIVFQueryParam().WithNProbe(32)
	if ivfQ.NProbe != 32 {
		t.Errorf("Expected NProbe=32, got %d", ivfQ.NProbe)
	}
}

func TestCollectionOptions(t *testing.T) {
	// Test CollectionOption
	opt := DefaultCollectionOption().
		WithReadOnly(true).
		WithCreateIfMissing(false).
		WithErrorIfExists(true)

	if !opt.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
	if opt.CreateIfMissing {
		t.Error("Expected CreateIfMissing=false")
	}
	if !opt.ErrorIfExists {
		t.Error("Expected ErrorIfExists=true")
	}

	// Test IndexOption
	idxOpt := DefaultIndexOption().WithAsync(true)
	if !idxOpt.Async {
		t.Error("Expected Async=true")
	}

	// Test OptimizeOption
	optOpt := DefaultOptimizeOption().WithFull(true)
	if !optOpt.Full {
		t.Error("Expected Full=true")
	}

	// Test AddColumnOption
	addColOpt := DefaultAddColumnOption().WithSkipBackfill(true)
	if !addColOpt.SkipBackfill {
		t.Error("Expected SkipBackfill=true")
	}

	// Test AlterColumnOption
	alterOpt := DefaultAlterColumnOption().WithSkipReindex(true)
	if !alterOpt.SkipReindex {
		t.Error("Expected SkipReindex=true")
	}
}

func TestStatus(t *testing.T) {
	// Test OK status
	okStatus := Status{Code: StatusCodeOK}
	if !okStatus.IsOK() {
		t.Error("Expected IsOK()=true for OK status")
	}

	// Test Failed status
	failStatus := Status{Code: StatusCodeFailed, Message: "test error"}
	if failStatus.IsOK() {
		t.Error("Expected IsOK()=false for Failed status")
	}
	if failStatus.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestCollectionStatsStruct(t *testing.T) {
	stats := &CollectionStats{
		DocCount:  100,
		SizeBytes: 1024000,
	}
	if stats.DocCount != 100 {
		t.Errorf("Expected DocCount=100, got %d", stats.DocCount)
	}
	if stats.SizeBytes != 1024000 {
		t.Errorf("Expected SizeBytes=1024000, got %d", stats.SizeBytes)
	}

	// Test String representation
	s := stats.String()
	if s == "" {
		t.Error("Expected non-empty String() representation")
	}
}
