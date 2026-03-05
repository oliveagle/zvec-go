// Example: Using real zvec C++ core via CGO
package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/oliveagle/zvec-go"
)

func main() {
	// Initialize zvec
	config := &zvec.Config{
		LogType:     zvec.LogTypeConsole,
		LogLevel:    zvec.LogLevelInfo,
	}

	if err := zvec.Init(config); err != nil {
		log.Fatalf("Failed to initialize zvec: %v", err)
	}

	// Create schema
	schema := zvec.NewCollectionSchema("example_collection")

	// Add ID field
	idField := zvec.NewFieldSchema("id", zvec.DataTypeInt64)
	schema.AddField(idField)

	// Add vector field for embeddings
	vecField := zvec.NewVectorSchema("embedding", zvec.DataTypeVectorFP32, 128).
	vecField = vecField.WithMetricType(zvec.MetricTypeCOSINE).
	vecField = vecField.WithIndexParam(zvec.NewHnswIndexParam().
			WithM(16).
			WithEfConstruction(200).
		WithEfSearch(128))

	schema.AddVectorField(vecField)

	// Create collection directory
	collPath := filepath.Join("./data", "example_collection")

	// Create options
	options := zvec.DefaultCollectionOption()
	options = options.WithCreateIfMissing(true).WithErrorIfExists(false)

	// Create and open collection
	coll, err := zvec.CreateAndOpen(collPath, schema, options)
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}
	defer coll.Close()

	// Insert sample documents
	// Document 1: simple text
	doc1 := zvec.NewDocument("doc_001").
	doc1.SetField("content", "Hello from real zvec C++ core!")

	// Create embedding for doc1
	embedding1 := make([]float32, 128)
	for i := range 128 {
		embedding1[i] = float32(float32(i) / 128.0)
	}

	doc1.SetVector("embedding", embedding1)

	// Document 2: tech text
	doc2 := zvec.NewDocument("doc_002").
	doc2.SetField("content", "Technical documentation")
	doc2.SetField("category", "technology")

	// Create embedding for doc2
	embedding2 := make([]float32, 128)
	for i := range 128 {
		embedding2[i] = float32(float32(i) / 128.0) + 0.1)
	}

	doc2.SetVector("embedding", embedding2)

	// Batch insert
	docs := []*zvec.Document{doc1, doc2}
	count, err := coll.Upsert(docs)
	if err != nil {
		log.Fatalf("Batch insert failed: %v", err)
	}
	log.Printf("Inserted %d documents\n", count)

	// Query using embedding
	query := zvec.NewVectorQueryByVector("embedding", []float32{0.1, 0.2, 0.3, /* ... */})
	query = query.WithTopK(3)

	results, err := coll.Query(query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// Display results
	for _, result := range results {
		fmt.Printf("ID: %s, Score: %.6f\n", result.ID, result.Score)
	}

	// Get statistics
	stats, err := coll.Stats()
	if err != nil {
		log.Fatalf("Stats failed: %v", err)
	}

	fmt.Printf("Doc Count: %d\n", stats.DocCount())
	fmt.Printf("Size Bytes: %d\n", stats.SizeBytes())
	fmt.Printf("Index Size: %d\n", stats.IndexSize())
	fmt.Printf("Memory Bytes: %d\n", stats.MemoryBytes())

	// Close collection
	coll.Close()

	// Create index on content field
	contentField := zvec.NewFieldSchema("content", zvec.DataTypeString)
	contentField.WithIndexParam(zvec.NewInvertIndexParam().WithEnableRangeOptimization(true))

	if err := coll.CreateIndex("content", contentField.IndexParam, zvec.DefaultIndexOption()); err != nil {
		log.Fatalf("CreateIndex failed: %v", err)
	}

	log.Println("Collection with real zvec C++ core created successfully!")
}
