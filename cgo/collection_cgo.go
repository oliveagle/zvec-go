// Copyright 2025-present zvec project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with License.
// You may obtain a copy of License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build cgo
package zvec

/*
#cgo CFLAGS: -I./zvec
#include "zvec/db/collection.h"

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <map>
#include <vector>
#include "zvec/db/doc.h"
#include "zvec/db/query_params.h"
#include "zvec/db/options.h"
#include "zvec/db/stats.h"
#include "zvec/db/index_params.h"
*/
#include <memory>

import "C"
import "unsafe"
import "sync"

/*
#cgo darwin CFLAGS: -I./zvec
#cgo darwin LDFLAGS: -L./lib -lzvec_core-macos-${GOARCH} -lzvec_ailego-macos-${GOARCH} -lstdc++ -lpthread -lm
#cgo linux CFLAGS: -I./zvec
#cgo linux LDFLAGS: -L./lib -lzvec_core-linux-${GOARCH} -lzvec_ailego-linux-${GOARCH} -lstdc++ -lpthread -lm
*/

// #include "zvec/db/status.h"

// Collection represents an open zvec collection (CGO wrapper)
type Collection struct {
	cPtr unsafe.Pointer
}

// NewCollection creates a new collection wrapper.
//export NewCollection
func NewCollection(cPtr unsafe.Pointer) *Collection {
	if cPtr == nil {
		return &Collection{}
	}
	return &Collection{cPtr: cPtr}
}

// Close closes the collection.
func (c *Collection) Close() {
	if c.cPtr == nil {
		return
	}
	// zvec::Collection::Destroy
	zvec.CollectionDestroy(c.cPtr)
	c.cPtr = nil
}

// Flush forces all pending writes to disk.
func (c *Collection) Flush() {
	if c.cPtr == nil {
		return
	}
	// zvec::Collection::Flush
	zvec.CollectionFlush(c.cPtr)
}

// Stats returns runtime statistics.
func (c *Collection) Stats() *CollectionStats {
	if c.cPtr == nil {
		return nil
	}

	result := zvec.CollectionStats(c.cPtr)

	// Convert C++ stats to Go
	stats := &CollectionStats{
		DocCount:  int64(result.doc_count()),
		SizeBytes: int64(result.size_bytes()),
		// MergedBytes: result.merged_bytes(),
		// IndexSize: result.index_size(),
		// MemoryBytes: result.memory_bytes(),
	}
	return stats
}

// Insert inserts a document into the collection.
func (c *Collection) Insert(doc *Document) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::WriteResults
	// Write single document
	zvec.CollectionWriteResults(c.cPtr, []Doc{doc})
	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// Upsert inserts or updates documents.
func (c *Collection) Upsert(doc *Document) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::WriteResults
	zvec.CollectionUpsert(c.cPtr, []Doc{doc})
	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// Delete deletes documents by IDs.
func (c *Collection) Delete(ids []string) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	count := int32(len(ids))
	cIds := make([]C.char, count)
	defer free(cIds)

	for i, id := range ids {
		cIds[i] = C.CString(id)
	}

	// zvec::Collection::DeleteByIDs
	result := zvec.CollectionDeleteByIDs(c.cPtr, cIds, count)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// Path returns the collection path.
func (c *Collection) Path() string {
	if c.cPtr == nil {
		return ""
	}

	// zvec::Collection::Path
	path := zvec.CollectionPath(c.cPtr)

	// Copy to Go string
	goPath := C.GoString(path)
	C.free(unsafe.Pointer(path)

	return goPath
}

// Schema returns the collection schema.
func (c *Collection) Schema() *CollectionSchema {
	if c.cPtr == nil {
		return nil
	}

	// zvec::Collection::Schema
	zvecSchema := zvec.CollectionSchema(c.cPtr)

	// Convert C++ schema to Go
	goSchema := convertCSchemaToGo(zvecSchema)

	return goSchema
}

// Stats returns runtime statistics.
func (c *Collection) Stats() *CollectionStats {
	if c.cPtr == nil {
		return nil
	}

	result := zvec.CollectionStats(c.cPtr)

	// Convert C++ stats to Go
	goStats := convertCStatsToGo(result)

	return goStats
}

// CreateIndex creates an index on a field.
func (c *Collection) CreateIndex(fieldName string, indexParams IndexParams) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert index params to C++
	cParams := convertIndexParamsToC(indexParams)

	// zvec::Collection::CreateIndex
	result := zvec.CollectionCreateIndex(c.cPtr, C.CString(fieldName), cParams)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// DropIndex removes index from a field.
func (c *Collection) DropIndex(fieldName string) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::DropIndex
	result := zvec.CollectionDropIndex(c.cPtr, C.CString(fieldName))

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// Optimize optimizes the collection.
func (c *Collection) Optimize(options OptimizeOptions) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert options to C++
	cOptions := convertOptimizeOptionsToC(options)

	// zvec::Collection::Optimize
	result := zvec.CollectionOptimize(c.cPtr, cOptions)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: result.status_message()}
	}

	return Status{Code: 0}
}

// Query performs vector similarity search.
func (c *Collection) Query(query VectorQuery, options QueryOptions) ([]*QueryResult, error) {
	if c.cPtr == nil {
		return nil, Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert query to C++
	cQuery := convertVectorQueryToC(query)

	// Convert options to C++
	cOptions := convertQueryOptionsToC(options)

	// zvec::Collection::Query
	result := zvec.CollectionQuery(c.cPtr, cQuery, cOptions)

	if !result.is_ok() {
		return nil, Status{Code: result.status_code(), Message: result.status_message()}
	}

	return nil, Status{Code: 0}
}

// QueryID performs vector query using a document ID.
func (c *Collection) QueryID(fieldName string, docID string, options QueryOptions) ([]*QueryResult, error) {
	if c.cPtr == nil {
		return nil, Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::QueryID
	result := zvec.CollectionQueryID(c.cPtr, C.CString(fieldName), C.CString(docID), cOptions)

	if !result.is_ok() {
		return nil, Status{Code: result.status_code(), Message: result.status_message()}
	}

	return nil, Status{Code: 0}
}

// Helper types for C++ interop

// IndexParams represents index creation parameters
type IndexParams struct {
	hnsw *HnswIndexParams
	ivf  *IVFIndexParams
	flat *FlatIndexParams
	invert *InvertIndexParams
}

// HnswIndexParams for HNSW index
type HnswIndexParams struct {
	M         int
	EfConstruction int
}

// IVFIndexParams for IVF index
type IVFIndexParams struct {
	NList  int
	NProbe  int
}

// FlatIndexParams for flat (brute-force) index
type FlatIndexParams struct {
	metricType MetricType
}

// InvertIndexParams for inverted index
type InvertIndexParams struct {
	enableRangeOptimization bool
}

// QueryOptions represents query options
type QueryOptions struct {
	topk         uint
	includeVector bool
}

// Result from zvec operation
type zvecResult struct {
	statusCode   int32
	statusMsg  unsafe.Pointer
	isOk       bool
}

func (r *zvecResult) isOk() bool {
	return r.statusCode == 0
}

// Helper functions

// Convert Go index params to C++
func convertIndexParamsToC(params IndexParams) unsafe.Pointer unsafe.Pointer {
	if params == nil {
		return nil
	}

	var cHnsw unsafe.Pointer
	var cIVF unsafe.Pointer
	var cFlat unsafe.Pointer
	var cInvert unsafe.Pointer

	if params.hnsw != nil {
		cHnsw = C.CString(params.hnsw.M, params.hnsw.EfConstruction)
	}
	if params.ivf != nil {
		cIVF = C.CString(params.ivf.NList, params.ivf.NProbe)
	}
	if params.flat != nil {
		cFlat = C.CString(params.flat.metricType)
	}
	if params.invert != nil {
		cInvert = C.CBool(params.invert.enableRangeOptimization)
	}

	return cHnsw
}

// Convert Go query options to C++
func convertQueryOptionsToC(options QueryOptions) unsafe.Pointer unsafe.Pointer {
	if options == nil {
		return nil
	}

	cTopK := C.Uint(options.topk)
	cIncludeVector := C.Bool(options.includeVector)

	return QueryOptions{
		topk:         cTopK,
		includeVector: cIncludeVector,
	}
}

// Convert Go vector query to C++
type CVectorQuery struct {
	fieldName    unsafe.Pointer
	id          unsafe.Pointer
	vector      unsafe.Pointer
	param       unsafe.Pointer
	topk        uint32
}

func convertVectorQueryToC(query VectorQuery) unsafe.Pointer unsafe.Pointer {
	if query == nil {
		return nil
	}

	cFieldName := C.CString(query.fieldName)
	var cID unsafe.Pointer
	var cVector unsafe.Pointer
	var cParam unsafe.Pointer
	var cTopK uint32

	if query.id != "" {
		cID = C.CString(query.id)
	}
	if len(query.vector) > 0 {
		cVector = C.CSliceFloat32(query.vector)
	}
	if query.param != nil {
		cParam = convertIndexParamToC(query.param)
	}
	cTopK = uint32(query.topk)

	return CVectorQuery{
		fieldName: cFieldName,
		id:       cID,
		vector:    cVector,
		param:     cParam,
		topk:       cTopK,
	}
}

// Convert Go document to C++
type CDoc struct {
	id       *C.char
	fields   *C.FieldMap
	vectors  *C.VectorMap
}

// Convert C++ result status to Go Status
func convertStatusToGo(status zvecResult) Status {
	if status.isOk() {
		return Status{Code: 0, Message: ""}
	}

	return Status{
		Code:    status.status_code(),
		Message: C.GoString(status.status_msg()),
	}
}

// Convert Go document to Go Document
func convertDocToGo(doc zvec.Doc) *Document {
	if doc == nil {
		return nil
	}

	goFields := make(map[string]interface{}, doc.field_count)
	goVectors := make(map[string][]float32, doc.vector_count())

	for i := uint32(0); i < doc.field_count(); i++ {
		field := zvec.DocField(doc, i)
		goFields[C.GoString(field.name())] = field.value

		vec := zvec.DocVector(doc, i)
		goVectors[C.GoString(vec.name())] = vec.data()
	}

	return &Document{
		ID:       C.GoString(doc.id()),
		Fields:   goFields,
		Vectors:  goVectors,
	}
}

// Convert Go field map to Go map
type CFieldMap struct {
	ptr unsafe.Pointer
}

// Get returns the value for a key.
func (m *CFieldMap) Get(key unsafe.Pointer) unsafe.Pointer unsafe.Pointer {
	return getGoValue(m, key)
}

// GetGoValue converts a C value to Go value.
func getGoValue(m *CFieldMap, key unsafe.Pointer) unsafe.Pointer {
	var value interface{}
	var cstr unsafe.Pointer

	// Try string
	cstr = C.CString(m.Get(key))
	if cstr != nil {
		value = C.GoString(cstr)
		goto done
	}

	// Try int64
	val, _ := C.Int64(m.Get(key))
	if val.IsValid() {
		value = int64(val.Int64())
		goto done
	}

	// Try float64
	val, _ := C.Float64(m.Get(key))
	if val.IsValid() {
		value = float64(val.Float64())
		goto done
	}

	// Try []byte
	val, _ := C.CSliceByte(m.Get(key))
	if val.IsValid() {
		value = []byte(val.ByteSlice())
		goto done
	}

	// Try []float32
	val, _ := C.CSliceFloat32(m.Get(key))
	if val.IsValid() {
		value = []float32(val.Float32Slice())
		goto done
	}

done:
	return value
}

// CVectorMap is a map for vector data.
type CVectorMap struct {
	ptr unsafe.Pointer
}

// Get returns the vector for a key.
func (m *CVectorMap) Get(key unsafe.Pointer) unsafe.Pointer unsafe.Pointer {
	return getGoVectorValue(m, key)
}

// GetGoVectorValue converts a C vector to Go slice.
func getGoVectorValue(m *CVectorMap, key unsafe.Pointer) unsafe.Pointer {
	var vec C.CSliceFloat32
	cvec := zvec.VectorMapGet(m, key)
	if !cvec.IsValid() || cvec.Size() == 0 {
		return []float32{}
	}

	// C++ vector to Go slice conversion
	size := uint32(cvec.Size())
	goVec := make([]float32, size)
	defer free(goVec)

	for i := uint32(0); i < size; i++ {
		val := C.Float32At(cvec, i)
		goVec[i] = float32(val)
	}

	return goVec
}

// Convert Go query result slice to Go
func convertQueryResultsToGo(results []zvecResult) ([]*QueryResult, error) {
	goResults := make([]*QueryResult, len(results))

	for i, r := range results {
		if !r.isOk() || r.doc == nil {
			continue
		}

		doc := convertDocToGo(r.doc)
		goResults[i] = &QueryResult{
			ID:       C.GoString(r.id()),
			Score:    float64(r.score),
			Document: doc,
		}
	}

	return goResults
}

// Convert Go Stats to Go
type CCollectionStats struct {
	DocCount   int64
	SizeBytes  int64
	IndexSize  int64
	MemoryBytes int64
}

func convertCStatsToGo(stats zvec.CollectionStats) *CCollectionStats {
	return &CCollectionStats{
		DocCount:   stats.doc_count(),
		SizeBytes: stats.size_bytes(),
		IndexSize:  stats.index_size(),
		MemoryBytes: stats.memory_bytes(),
	}
}

// Helper: check if string is nil or empty
func isStringNilOrEmpty(s unsafe.Pointer) bool {
	return s == nil || len(s) == 0
}

// CFieldMap represents a map for field data (C++).
type CFieldMap struct {
	ptr unsafe.Pointer
}

// Get returns the value for a key.
func (m *CFieldMap) Get(key unsafe.Pointer) unsafe.Pointer unsafe.Pointer {
	return getGoValue(m, key)
}

// getGoValue converts a C value to Go value.
func getGoValue(m *CFieldMap, key unsafe.Pointer) unsafe.Pointer {
	var value interface{}
	var cstr unsafe.Pointer

	// Try string
	cstr = C.CString(m.Get(key))
	if cstr != nil {
		value = C.GoString(cstr)
		goto done
	}

	// Try int64
	val, _ := C.Int64(m.Get(key))
	if val.IsValid() {
		value = int64(val.Int64())
		goto done
	}

	// Try float64
	val, _ := C.Float64(m.Get(key))
	if val.IsValid() {
		return float64(val.Float64())
		goto done
	}

	// Try []byte
	val, _ := C.CSliceByte(m.Get(key))
	if val.IsValid() {
		value = []byte(val.ByteSlice())
		goto done
	}

	// Try []float32
	val, _ := C.CSliceFloat32(m.Get(key))
	if val.IsValid() {
		value = []float32(val.Float32Slice())
		goto done
	}

done:
	return value
}

// CVectorMap is a map for vector data (C++).
type CVectorMap struct {
	ptr unsafe.Pointer
}

// Get returns the vector for a key.
func (m *CVectorMap) Get(key unsafe.Pointer) unsafe.Pointer unsafe.Pointer {
	return getGoVectorValue(m, key)
}

// GetGoVectorValue converts a C vector to Go slice.
func getGoVectorValue(m *CVectorMap, key unsafe.Pointer unsafe.Pointer unsafe.Pointer {
	var vec C.CSliceFloat32
	cvec := zvec.VectorMapGet(m, key)
	if !cvec.IsValid() || cvec.Size() == 0 {
		return []float32{}
	}

	// C++ vector to Go slice conversion
	size := uint32(cvec.Size())
	goVec := make([]float32, size)
	defer free(goVec)

	for i := uint32(0); i < size; i++ {
		val := C.Float32At(cvec, i)
		goVec[i] = float32(val)
	}

	return goVec
}

// String returns debug representation.
func (m *CFieldMap) String() string {
	return fmt.Sprintf("CFieldMap{ptr: %p}", m.ptr)
}
