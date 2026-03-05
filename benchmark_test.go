package zvec

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func BenchmarkDocumentSetField(b *testing.B) {
	doc := NewDocument("bench-doc")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.SetField(fmt.Sprintf("field%d", i%100), "value")
	}
}

func BenchmarkDocumentSetVector(b *testing.B) {
	doc := NewDocument("bench-doc")
	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.SetVector("embedding", vec)
	}
}

func BenchmarkCollectionInsert(b *testing.B) {
	// Setup
	globalZvec = nil
	once = sync.Once{}
	tmpDir, err := os.MkdirTemp("", "zvec-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(DefaultConfig()); err != nil {
		b.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("bench")
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 128))
	collPath := filepath.Join(tmpDir, "bench_coll")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		b.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Prepare documents
	docs := make([]*Document, b.N)
	for i := 0; i < b.N; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		vec := make([]float32, 128)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		doc.SetVector("embedding", vec)
		docs[i] = doc
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := coll.Insert(docs[i]); err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}
}

func BenchmarkCollectionInsertBatch(b *testing.B) {
	batchSizes := []int{10, 100, 1000}
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			// Setup
			globalZvec = nil
			once = sync.Once{}
			tmpDir, err := os.MkdirTemp("", "zvec-bench-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := Init(DefaultConfig()); err != nil {
				b.Fatalf("Init failed: %v", err)
			}

			schema := NewCollectionSchema("bench")
			schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 128))
			collPath := filepath.Join(tmpDir, "bench_coll")
			coll, err := CreateAndOpen(collPath, schema, nil)
			if err != nil {
				b.Fatalf("CreateAndOpen failed: %v", err)
			}
			defer coll.Close()

			// Prepare batches
			numBatches := b.N
			batches := make([][]*Document, numBatches)
			for i := 0; i < numBatches; i++ {
				batch := make([]*Document, batchSize)
				for j := 0; j < batchSize; j++ {
					doc := NewDocument(fmt.Sprintf("doc_%d_%d", i, j))
					vec := make([]float32, 128)
					for k := range vec {
						vec[k] = rand.Float32()
					}
					doc.SetVector("embedding", vec)
					batch[j] = doc
				}
				batches[i] = batch
			}

			b.ResetTimer()
			for i := 0; i < numBatches; i++ {
				if _, err := coll.InsertBatch(batches[i]); err != nil {
					b.Fatalf("InsertBatch failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkCollectionGet(b *testing.B) {
	// Setup
	globalZvec = nil
	once = sync.Once{}
	tmpDir, err := os.MkdirTemp("", "zvec-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(DefaultConfig()); err != nil {
		b.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("bench")
	schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 128))
	collPath := filepath.Join(tmpDir, "bench_coll")
	coll, err := CreateAndOpen(collPath, schema, nil)
	if err != nil {
		b.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	// Insert 1000 documents
	numDocs := 1000
	for i := 0; i < numDocs; i++ {
		doc := NewDocument(fmt.Sprintf("doc%d", i))
		vec := make([]float32, 128)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		doc.SetVector("embedding", vec)
		if err := coll.Insert(doc); err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docID := fmt.Sprintf("doc%d", i%numDocs)
		if _, err := coll.Get(docID); err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func BenchmarkCollectionSearch(b *testing.B) {
	dimensions := []int{64, 128, 256, 512}
	collectionSizes := []int{100, 1000, 10000}

	for _, dim := range dimensions {
		for _, size := range collectionSizes {
			b.Run(fmt.Sprintf("Dim_%d_Size_%d", dim, size), func(b *testing.B) {
				// Setup
				globalZvec = nil
				once = sync.Once{}
				tmpDir, err := os.MkdirTemp("", "zvec-bench-*")
				if err != nil {
					b.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tmpDir)

				if err := Init(DefaultConfig()); err != nil {
					b.Fatalf("Init failed: %v", err)
				}

				schema := NewCollectionSchema("bench")
				schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, dim))
				collPath := filepath.Join(tmpDir, "bench_coll")
				coll, err := CreateAndOpen(collPath, schema, nil)
				if err != nil {
					b.Fatalf("CreateAndOpen failed: %v", err)
				}
				defer coll.Close()

				// Insert documents
				for i := 0; i < size; i++ {
					doc := NewDocument(fmt.Sprintf("doc%d", i))
					vec := make([]float32, dim)
					for j := range vec {
						vec[j] = rand.Float32()
					}
					doc.SetVector("embedding", vec)
					if err := coll.Insert(doc); err != nil {
						b.Fatalf("Insert failed: %v", err)
					}
				}

				// Prepare query vectors
				queryVecs := make([][]float32, b.N)
				for i := 0; i < b.N; i++ {
					vec := make([]float32, dim)
					for j := range vec {
						vec[j] = rand.Float32()
					}
					queryVecs[i] = vec
				}

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					query := NewVectorQueryByVector("embedding", queryVecs[i]).WithTopK(10)
					if _, err := coll.Search(query); err != nil {
						b.Fatalf("Search failed: %v", err)
					}
				}
			})
		}
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	dimensions := []int{64, 128, 256, 512, 1024}
	for _, dim := range dimensions {
		b.Run(fmt.Sprintf("Dim_%d", dim), func(b *testing.B) {
			a := make([]float32, dim)
			bvec := make([]float32, dim)
			for i := range a {
				a[i] = rand.Float32()
				bvec[i] = rand.Float32()
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = cosineSimilarity(a, bvec)
			}
		})
	}
}

func BenchmarkSchemaValidation(b *testing.B) {
	schema := NewCollectionSchema("bench")
	for i := 0; i < 10; i++ {
		schema.AddField(NewFieldSchema(fmt.Sprintf("field%d", i), DataTypeString))
	}
	for i := 0; i < 5; i++ {
		schema.AddVectorField(NewVectorSchema(fmt.Sprintf("vec%d", i), DataTypeVectorFP32, 128))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schema.Validate()
	}
}

func BenchmarkVectorQueryValidation(b *testing.B) {
	query := NewVectorQueryByVector("embedding", make([]float32, 128))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = query.Validate()
	}
}

// Integration benchmarks

func BenchmarkEndToEndWorkflow(b *testing.B) {
	// Measures complete workflow: create collection -> insert -> search -> get
	b.StopTimer()
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < b.N; i++ {
		func() {
			// Setup
			globalZvec = nil
			once = sync.Once{}
			tmpDir, err := os.MkdirTemp("", "zvec-e2e-*")
			if err != nil {
				b.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			b.StartTimer()

			// 1. Init
			if err := Init(DefaultConfig()); err != nil {
				b.Fatalf("Init failed: %v", err)
			}

			// 2. Create schema and collection
			schema := NewCollectionSchema("e2e_test")
			schema.AddField(NewFieldSchema("id", DataTypeInt64))
			schema.AddVectorField(NewVectorSchema("embedding", DataTypeVectorFP32, 128))

			collPath := filepath.Join(tmpDir, "e2e_coll")
			coll, err := CreateAndOpen(collPath, schema, nil)
			if err != nil {
				b.Fatalf("CreateAndOpen failed: %v", err)
			}

			// 3. Insert 100 documents
			for j := 0; j < 100; j++ {
				doc := NewDocument(fmt.Sprintf("doc%d", j))
				doc.SetField("id", j)
				vec := make([]float32, 128)
				for k := range vec {
					vec[k] = rand.Float32()
				}
				doc.SetVector("embedding", vec)
				if err := coll.Insert(doc); err != nil {
					b.Fatalf("Insert failed: %v", err)
				}
			}

			// 4. Search
			queryVec := make([]float32, 128)
			for j := range queryVec {
				queryVec[j] = rand.Float32()
			}
			query := NewVectorQueryByVector("embedding", queryVec).WithTopK(10)
			if _, err := coll.Search(query); err != nil {
				b.Fatalf("Search failed: %v", err)
			}

			// 5. Get a document
			if _, err := coll.Get("doc50"); err != nil {
				b.Fatalf("Get failed: %v", err)
			}

			// 6. Close
			if err := coll.Close(); err != nil {
				b.Fatalf("Close failed: %v", err)
			}

			b.StopTimer()
		}()
	}
}
