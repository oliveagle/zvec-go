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
