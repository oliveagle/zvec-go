package zvec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestCollectionConcurrentAccess hammers one collection from many goroutines:
// writers, readers, and a deleter, to expose lock and map-iteration races
// under -race.
func TestCollectionConcurrentAccess(t *testing.T) {
	globalZvec = nil
	once = sync.Once{}

	tmpDir, err := os.MkdirTemp("", "zvec-stress-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(DefaultConfig()); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	schema := NewCollectionSchema("stress")
	schema.AddField(NewFieldSchema("n", DataTypeInt64))
	schema.AddVectorField(NewVectorSchema("emb", DataTypeVectorFP32, 4))

	coll, err := CreateAndOpen(filepath.Join(tmpDir, "stress"), schema, nil)
	if err != nil {
		t.Fatalf("CreateAndOpen failed: %v", err)
	}
	defer coll.Close()

	var wg sync.WaitGroup
	const (
		writers      = 8
		readers      = 8
		perGoroutine = 50
	)

	// Writers: each owns a disjoint ID range so cross-writer overwrites are
	// deterministic per writer, while sharing the collection's maps/locks.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				doc := NewDocument(fmt.Sprintf("w%d-%d", w, i))
				doc.SetField("n", int64(i))
				doc.SetVector("emb", []float32{float32(w), 0, 0, 0})
				if err := coll.Upsert(doc); err != nil {
					t.Errorf("Upsert: %v", err)
					return
				}
			}
		}(w)
	}

	// Readers: search + list + get + count concurrently.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := coll.Search(NewVectorQueryByVector("emb", []float32{float32(r), 0, 0, 0}).WithTopK(5)); err != nil {
					t.Errorf("Search: %v", err)
					return
				}
				if _, err := coll.ListDocs(0, 10); err != nil {
					t.Errorf("ListDocs: %v", err)
					return
				}
				if _, err := coll.Get(fmt.Sprintf("w%d-%d", r%writers, i)); err != nil && !errors.Is(err, ErrDocNotFound) {
					t.Errorf("Get: %v", err)
					return
				}
				if _, err := coll.Count(); err != nil {
					t.Errorf("Count: %v", err)
					return
				}
			}
		}(r)
	}

	// Deleter: removes a disjoint range mid-flight.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < perGoroutine; i++ {
			_ = coll.Delete(fmt.Sprintf("w%d-%d", writers, i)) // writers index == deleter's range, never written
		}
	}()

	// Query-by-ID path (in-memory-first lookup) under load.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < perGoroutine; i++ {
			q := NewVectorQueryByID("emb", fmt.Sprintf("w0-%d", i))
			if _, err := coll.Query(q, 3, "", false, nil); err != nil {
				// The document may not exist yet; any other error is a real bug.
				if !errors.Is(err, ErrDocNotFound) {
					t.Errorf("Query by ID: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()

	n, err := coll.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	want := int64(writers * perGoroutine)
	if n < want-1 || n > want {
		t.Errorf("final doc count = %d, want ~%d", n, want)
	}

}
