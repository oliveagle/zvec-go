# zvec-go

[![Go Reference](https://pkg.go.dev/badge/github.com/oliveagle/zvec-go.svg)](https://pkg.go.dev/github.com/oliveagle/zvec-go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Go client for [zvec](https://github.com/b	tracepc/zvec) - a high-performance vector database.

## Features

- **Complete Go API** - Consistent with Python API style
- **Schema Definition** - Flexible scalar and vector field definitions
- **Multiple Index Support** - HNSW/IVF/Flat index parameter configuration
- **Vector Search** - Cosine/L2/IP distance metrics
- **Batch Operations** - High-performance bulk insert and retrieval
- **Thread Safe** - Built-in concurrency safety

## Installation

```bash
go get github.com/oliveagle/zvec-go
```

## Prerequisites

zvec uses CGO to interface with the C++ library. You need:

1. **zvec library** installed on your system
2. **CGO enabled** (`CGO_ENABLED=1`)

### Installing zvec

```bash
# Clone zvec
git clone https://github.com/tracepc/zvec.git
cd zvec

# Build
mkdir build && cd build
cmake ..
make -j$(nproc)
sudo make install
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/oliveagle/zvec-go"
)

func main() {
    // 1. Initialize
    zvec.Init(zvec.DefaultConfig())

    // 2. Create Schema
    schema := zvec.NewCollectionSchema("my_collection")
    schema.AddField(zvec.NewFieldSchema("id", zvec.DataTypeInt64))
    schema.AddVectorField(
        zvec.NewVectorSchema("embedding", zvec.DataTypeVectorFP32, 128).
            WithMetricType(zvec.MetricTypeCOSINE),
    )

    // 3. Create Collection
    coll, err := zvec.CreateAndOpen("./data/my_collection", schema, nil)
    if err != nil {
        panic(err)
    }
    defer coll.Close()

    // 4. Insert Documents
    doc := zvec.NewDocument("doc_001").
        SetField("id", int64(1)).
        SetVector("embedding", []float32{0.1, 0.2, /* ... */ })
    coll.Insert(doc)

    // 5. Vector Search
    query := zvec.NewVectorQueryByVector("embedding", queryVector).WithTopK(5)
    results, err := coll.Search(query)
    if err != nil {
        panic(err)
    }

    for _, result := range results {
        fmt.Printf("ID: %s, Score: %f\n", result.ID, result.Score)
    }
}
```

## API Reference

### Initialization

```go
// Default configuration
zvec.Init(zvec.DefaultConfig())

// Custom configuration
config := &zvec.Config{
    LogLevel: zvec.LogLevelInfo,
}
zvec.Init(config)
```

### Schema Definition

```go
schema := zvec.NewCollectionSchema("collection_name")

// Add scalar fields
schema.AddField(zvec.NewFieldSchema("id", zvec.DataTypeInt64))
schema.AddField(zvec.NewFieldSchema("name", zvec.DataTypeString))
schema.AddField(zvec.NewFieldSchema("price", zvec.DataTypeFloat))

// Add vector field
schema.AddVectorField(
    zvec.NewVectorSchema("embedding", zvec.DataTypeVectorFP32, 128).
        WithMetricType(zvec.MetricTypeCOSINE).
        WithIndexType(zvec.IndexTypeHNSW),
)
```

### Collection Operations

```go
// Create and open
coll, _ := zvec.CreateAndOpen(path, schema, nil)

// Open existing
coll, _ := zvec.Open(path)

// Close
coll.Close()

// Insert
coll.Insert(doc)

// Batch insert
coll.BatchInsert([]zvec.Document{doc1, doc2, doc3})

// Search
results, _ := coll.Search(query)

// Get
doc, _ := coll.Get("doc_id")

// Delete
coll.Delete("doc_id")
```

### Data Types

| Type | Description |
|------|-------------|
| `DataTypeBool` | Boolean |
| `DataTypeInt8` | 8-bit integer |
| `DataTypeInt16` | 16-bit integer |
| `DataTypeInt32` | 32-bit integer |
| `DataTypeInt64` | 64-bit integer |
| `DataTypeFloat` | 32-bit float |
| `DataTypeDouble` | 64-bit float |
| `DataTypeString` | String |
| `DataTypeVectorFP32` | 32-bit float vector |
| `DataTypeVectorFP16` | 16-bit float vector |
| `DataTypeVectorBF16` | BFloat16 vector |
| `DataTypeVectorInt8` | 8-bit integer vector |

### Distance Metrics

| Metric | Description |
|--------|-------------|
| `MetricTypeL2` | Euclidean distance |
| `MetricTypeIP` | Inner product |
| `MetricTypeCOSINE` | Cosine similarity |

### Index Types

| Index | Description |
|-------|-------------|
| `IndexTypeFlat` | Brute-force search |
| `IndexTypeHNSW` | Hierarchical Navigable Small World |
| `IndexTypeIVF` | Inverted File Index |

## Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [zvec](https://github.com/tracepc/zvec) - The underlying C++ vector database
