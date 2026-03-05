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
*/

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <map>
#include <vector>

import "C"
import "unsafe"
import "sync"

/*
#cgo LDFLAGS: -I./zvec
-L/usr/local/lib -L/usr/local/lib/x86_64-linux-gnu
*/

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
	c.cPtr = nil
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
		IndexSize: result.index_size(),
		MemoryBytes: result.memory_bytes(),
	}
	return stats
}

// Insert inserts a document into the collection.
func (c *Collection) Insert(doc *Document) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert Go document to C++
	cDoc := convertDocToC(doc)
	// zvec::Collection::WriteResults
	result := zvec.CollectionWrite(c.cPtr, []Doc{cDoc})
	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// Upsert inserts or updates documents.
func (c *Collection) Upsert(doc *Document) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert Go document to C++
	cDoc := convertDocToC(doc)
	// zvec::Collection::WriteResults
	result := zvec.CollectionUpsert(c.cPtr, []Doc{cDoc})
	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// Delete deletes documents by IDs.
func (c *Collection) Delete(ids []string) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert Go string IDs to C++
	cIds := make([]C.String, len(ids))
	defer free(cIds)

	for i, id := range ids {
		cIds[i] = C.CString(id)
	}

	// zvec::Collection::DeleteByIDs
	result := zvec.CollectionDeleteByIDs(c.cPtr, cIds, uint64(len(ids)))
	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// CreateIndex creates an index on a field.
func (c *Collection) CreateIndex(fieldName string, params IndexParams) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert params to C++
	cParams := convertIndexParamsToC(params)

	// zvec::Collection::CreateIndex
	result := zvec.CollectionCreateIndex(c.cPtr, C.CString(fieldName), cParams)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// DropIndex removes an index from a field.
func (c *Collection) DropIndex(fieldName string) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::DropIndex
	result := zvec.CollectionDropIndex(c.cPtr, C.CString(fieldName))

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
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
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// AddColumn adds a new column to the collection.
func (c *Collection) AddColumn(fieldSchema FieldSchema, expression string, options AddColumnOptions) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert field schema to C++
	cFieldSchema := convertFieldSchemaToC(fieldSchema)

	// zvec::Collection::AddColumn
	result := zvec.CollectionAddColumn(c.cPtr, cFieldSchema, C.CString(expression), cOptions)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// DropColumn removes a column from the collection.
func (c *Collection) DropColumn(fieldName string) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

// zvec::Collection::DropColumn
	result := zvec.CollectionDropColumn(c.cPtr, C.CString(fieldName))

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// AlterColumn modifies a column or updates its schema.
func (c *Collection) AlterColumn(oldName string, newName string, fieldSchema FieldSchema, options AlterColumnOptions) Status {
	if c.cPtr == nil {
		return Status{Code: -1, Message: "collection pointer is nil"}
	}

	// zvec::Collection::AlterColumn
	result := zvec.CollectionAlterColumn(c.cPtr, C.CString(oldName), C.CString(newName), cFieldSchemaPtr)

	if !result.is_ok() {
		return Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return Status{Code: 0}
}

// Query performs a vector similarity search.
func (c *Collection) Query(query VectorQuery, options QueryOptions) ([]*QueryResult, error) {
	if c.cPtr == nil {
		return nil, Status{Code: -1, Message: "collection pointer is nil"}
	}

	// Convert query and options to C++
	cQuery := convertVectorQueryToC(query)
	cOptions := convertQueryOptionsToC(options)

	// zvec::Collection::Query
	result := zvec.CollectionQuery(c.cPtr, cQuery, cOptions)

	if !result.is_ok() {
		return nil, Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return result
}

// QueryID performs a vector query using a document ID.
func (c *Collection) QueryID(fieldName string, docID string, options QueryOptions) ([]*QueryResult, error) {
	if c.cPtr == nil {
		return nil, Status{Code: -1, Message: "collection pointer is nil"}
	}

	cDocID := C.CString(docID)

	// zvec::Collection::QueryID
	result := zvec.CollectionQueryID(c.cPtr, C.CString(fieldName), cDocID, cOptions)

	if !result.is_ok() {
		return nil, Status{Code: result.status_code(), Message: C.GoString(result.status_msg())}
	}

	return result
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

	C.free(unsafe.Pointer(path))

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
	return convertCSchemaToGo(zvecSchema)
}

// CollectionStats represents runtime statistics.
type CollectionStats struct {
	DocCount   int64
	SizeBytes int64
	MergedBytes int64
	IndexSize  int64
	MemoryBytes int64
}

// IndexParams represents index creation parameters.
type IndexParams struct {
	Hnsw *HnswIndexParams
	IVF  *IVFIndexParams
	Flat *FlatIndexParams
	Invert *InvertIndexParams
}

// HnswIndexParams for HNSW index
type HnswIndexParams struct {
	M              int
	EfConstruction int
	EfSearch       int
}

// IVFIndexParams for IVF index
type IVFIndexParams struct {
	NList  int
	NProbe int
}

// FlatIndexParams for flat (brute-force) index
type FlatIndexParams struct {
	MetricType MetricType
}

// InvertIndexParams for inverted index
type InvertIndexParams struct {
	EnableRangeOptimization bool
}

// OptimizeOptions represents optimize options.
type OptimizeOptions struct {
	Full bool
}

// QueryOptions represents query options.
type QueryOptions struct {
	TopK         uint
	IncludeVector bool
}

// AddColumnOptions represents column add options.
type AddColumnOptions struct {
	SkipBackfill bool
}

// AlterColumnOptions represents column alter options.
type AlterColumnOptions struct {
	SkipReindex bool
}

// Query represents a vector search query.
type VectorQuery struct {
	fieldName unsafe.Pointer
	id          unsafe.Pointer
	vector     unsafe.Pointer
	param       unsafe.Pointer
	topk        uint32
}

// CDocument represents a C++ document handle.
type CDocument struct {
	cDoc unsafe.Pointer
}

// Document represents a document (Go wrapper with C++ backing).
type Document struct {
	cDoc unsafe.Pointer
}

// Helper type for C++ string conversion
type CString struct {
	cStr unsafe.Pointer
}

// NewCString creates a CString from Go string.
func NewCString(goStr string) *CString {
	cStr := C.CString(goStr)
	return &CString{cStr: cStr}
}

func (s *CString) String() string {
	return C.GoString(s.cStr)
}

// Helper functions for C++ interop

// C.String returns a C++ string (must call C.free after use)
func CString(s *CString) string {
	return C.GoString(s.String)
}

// C.free calls C.stdlib free on the C string.
func C.free(s *CString) {
	C.stdlib.free(s.String())
}

// C.GoString converts to Go string (must call C.free after use).
func (s *CString) GoString() string {
	result := C.stdlib.GoString(s.String())
	C.stdlib.free(s.String())
	return result
}

// C.GoString converts to C++ string (must copy, not use after C.free).
func (s *CString) GoString() string {
	// Create a copy since C.stdlib.GoString doesn't copy
	result := C.stdlib.C.GoString(s.String())
	C.stdlib.free(s.String())

	return result
}

// Helper function: check if string is nil or empty.
func isStringNilOrEmpty(s unsafe.Pointer) bool {
	return s == nil || len(s) == 0
}

// C++ Result helper structure
type cResult struct {
	status_code   int32
	status_msg   unsafe.Pointer
}

// checkResult checks if a C++ result indicates success.
func checkResult(result cResult) bool {
	return result.status_code() == 0
}

// checkResultMsg returns the status message from C++ result.
func checkResultMsg(result cResult) string {
	return C.GoString(result.status_msg())
}

// Helper type: CVec for float32 vector
type CVec struct {
	ptr unsafe.Pointer
	len int
}

// NewCVec creates a CVec from Go slice.
func NewCVec(goVec []float32) *CVec {
	ptr := C.CFloat32Slice(goVec)
	if ptr == nil {
		return CVec{}
	}

	return &CVec{ptr: ptr, len: len(goVec)}
}

// Free releases the CVec pointer.
func (v *CVec) Free() {
	if v.ptr == nil {
		return
	}
	C.stdlib.Free(v.ptr)
	v.ptr = nil
}

// Data returns the vector data slice.
func (v *CVec) Data() []float32 {
	if v.ptr == nil {
		return nil
	}
	return C.stdlib.GoStringSlice(v.ptr, v.len)
}

// Helper functions: convert C++ types to C

// C.String converts a Go string to C++ string.
// After use, must call C.free() to free memory.
func C.GoString(goStr string) *CString {
	s.cStr = C.CString(goStr)
	return s.cStr
}

// C.GoString converts to C++ string to Go string.
func (s *CString) GoString() string {
	result := C.stdlib.GoString(s.String())
	C.stdlib.free(s.String())
	return result
}

// convertCParamsToC converts Go IndexParams to C++ IndexParams.
func convertIndexParamsToC(params IndexParams) unsafe.Pointer unsafe.Pointer {
	if params == nil {
		return IndexParams{}
	}

	var cHnsw unsafe.Pointer
	var cIVF unsafe.Pointer
	var cFlat unsafe.Pointer
	var cInvert unsafe.Pointer

	// Convert HnswIndexParams
	if params.hnsw != nil {
		cHnsw = C.HnswIndexParams{
			M:              C.Int32(params.hnsw.M),
			EfConstruction: C.Int32(params.hnsw.EfConstruction),
			EfSearch:       C.Int32(params.hnsw.EfSearch),
		}
	}

	// Convert IVFIndexParams
	if params.ivf != nil {
		cIVF = C.IVFIndexParams{
			NList: C.Int32(params.ivf.NList),
			NProbe: C.Int32(params.ivf.NProbe),
		}
	}

	// Convert FlatIndexParams
	if params.flat != nil {
		cFlat = C.FlatIndexParams{
			MetricType: convertMetricTypeToC(params.flat.MetricType),
		}
	}

	// Convert InvertIndexParams
	if params.invert != nil {
		cInvert = C.InvertIndexParams{
			EnableRangeOptimization: C.Bool(params.invert.EnableRangeOptimization),
		}
	}

	// IndexParams combines all index types.
type IndexParams struct {
	hnsw  unsafe.Pointer
	ivf  unsafe.Pointer
	flat unsafe.Pointer
	invert unsafe.Pointer
}

// convertIndexParamsToC creates C++ IndexParams from Go IndexParams.
func convertIndexParamsToC(params IndexParams) unsafe.Pointer unsafe.Pointer {
	if params == nil {
		return IndexParams{}
	}

	return IndexParams{
		hnsw:  cHnsw,
		ivf: cIVF,
		flat: cFlat,
		invert: cInvert,
	}
}

// convertQueryOptionsToC creates C++ QueryOptions from Go QueryOptions.
func convertQueryOptionsToC(options QueryOptions) unsafe.Pointer unsafe.Pointer {
	if options == nil {
		return QueryOptions{}
	}

	return QueryOptions{
		TopK:         C.Uint32(options.TopK),
		IncludeVector: C.Bool(options.IncludeVector),
	}
}

// convertVectorQueryToC creates C++ VectorQuery from Go VectorQuery.
func convertVectorQueryToC(query VectorQuery) unsafe.Pointer unsafe.Pointer {
	if query == nil {
		return VectorQuery{}
	}

	return VectorQuery{
		fieldName: C.GoString(query.FieldName),
		id:       c.GoStringOrNil(query.ID),
		vector:   newCVec(convertGoSliceToC(query.Vector)),
		param:     convertIndexParamToC(query.Param),
		topk:      C.Uint32(query.TopK),
	}
}

// Helper functions

// checkCResult checks if a C++ result indicates success.
func checkCResult(result cResult) bool {
	return result.status_code() == 0
}

// checkCResultMsg returns the status message from C++ result.
func checkCResultMsg(result cResult) string {
	return C.GoString(result.status_msg())
}

// convertCSchemaToGo converts C++ CollectionSchema to Go CollectionSchema.
func convertCSchemaToGo(schema zvec.CollectionSchema) *CollectionSchema {
	// Convert fields
	goFields := make([]CFieldMap, len(schema.Fields))
	defer free(goFields)

	for i, field := range schema.Fields {
		goFields[i] = convertFieldMapToC(field)
	}

	// Convert vector fields
	goVectors := make([]CVecMap, len(schema.VectorFields))
	defer free(goVectors)

	for i, vec := range schema.VectorFields {
		goVectors[i] = convertVectorSchemaToC(vec)
	}

	// Convert options to C++
	var cOptions unsafe.Pointer

	if schema.Option != nil {
		cOptions = convertCollectionOptionsToC(schema.Option)
	}

	// Create collection with converted schema
	cColl, err := NewCollection(schema)

	return cColl, err
}

// convertFieldMapToC converts a Go FieldSchema to C++ FieldSchema.
func convertFieldMapToC(fieldSchema zvec.CollectionSchema) *CFieldSchema *CFieldSchema {
	if fieldSchema == nil {
		return CFieldSchema{}
	}

	return &CFieldSchema{
		name:     C.GoString(fieldSchema.Name),
		dataType: convertDataTypeToC(fieldSchema.DataType),
		nullable:  C.Bool(fieldSchema.Nullable),
		indexParam: convertInvertIndexParamToC(fieldSchema.IndexParam),
	}
}

// convertVectorSchemaToC converts a Go VectorSchema to C++ VectorSchema.
func convertVectorSchemaToC(vecSchema zvec.VectorSchema) *CVectorSchema {
	if vecSchema == nil {
		return CVectorSchema{}
	}

	return &CVectorSchema{
		name:       C.GoString(vecSchema.Name),
		dataType: convertDataTypeToC(vecSchema.DataType),
		dimension:  int32(vecSchema.Dimension),
		metricType: convertMetricTypeToC(vecSchema.MetricType),
		indexParam: convertIndexParamToC(vecSchema.IndexParam),
	}
}

// convertIndexParamToC converts a Go index param to C++ IndexParam.
func convertIndexParamToC(param interface{}) unsafe.Pointer unsafe.Pointer {
	switch p := param.(type) {
	case *HnswIndexParams:
		return &HnswIndexParams{
			M:              C.Int32(p.M),
			EfConstruction: C.Int32(p.EfConstruction),
			EfSearch:       C.Int32(p.EfSearch),
		}
	case *IVFIndexParams:
		return &IVFIndexParams{
			NList: C.Int32(p.NList),
			NProbe: C.Int32(p.NProbe),
		}
	case *FlatIndexParams:
		return &FlatIndexParams{
			MetricType: convertMetricTypeToC(p.MetricType),
		}
	case *InvertIndexParams:
		return &InvertIndexParams{
			EnableRangeOptimization: C.Bool(p.EnableRangeOptimization),
		}
	case nil:
		return nil
	}
}

// convertCollectionOptionsToC converts Go CollectionOption to C++ CollectionOptions.
func convertCollectionOptionsToC(option zvec.CollectionOption) unsafe.Pointer unsafe.Pointer {
	if option == nil {
		return CollectionOptions{}
	}

	return &CollectionOptions{
		ReadOnly: C.Bool(option.ReadOnly),
	}
}

// convertOptimizeOptionsToC converts Go OptimizeOptions to C++ OptimizeOptions.
func convertOptimizeOptionsToC(options OptimizeOptions) unsafe.Pointer unsafe.Pointer {
	if options == nil {
		return OptimizeOptions{}
	}

	return &OptimizeOptions{
		Full: C.Bool(options.Full),
	}

// convertAddColumnOptionsToC converts Go AddColumnOptions to C++ AddColumnOptions.
func convertAddColumnOptionsToC(options AddColumnOptions) unsafe.Pointer unsafe.Pointer {
	if options == nil {
		return AddColumnOptions{}
	}

	return &AddColumnOptions{
		SkipBackfill: C.Bool(options.SkipBackfill),
	}

// convertAlterColumnOptionsToC converts Go AlterColumnOptions to C++ AlterColumnOptions.
func convertAlterColumnOptionsToC(options AlterColumnOptions) unsafe.Pointer unsafe.Pointer {
	if options == nil {
		return AlterColumnOptions{}
	}

	return &AlterColumnOptions{
		SkipReindex: C.Bool(options.SkipReindex),
	}

// Helper types for C++ collections

// CVec represents a C++ vector.
type CVec struct {
	ptr unsafe.Pointer
	len int
}

// NewCVec creates a CVec from Go slice.
func NewCVec(goVec []float32) *CVec {
	ptr := C.CFloat32Slice(goVec)
	if ptr == nil {
		return CVec{}
	}
	return &CVec{ptr: ptr, len: len(goVec)}
}

// Free releases the CVec pointer.
func (v *CVec) Free() {
	if v.ptr == nil {
		return
	}
	C.stdlib.Free(v.ptr)
	v.ptr = nil
}

// Data returns the vector data slice.
func (v *CVec) Data() []float32 {
	if v.ptr == nil {
		return nil
	}
	return C.stdlib.GoStringSlice(v.ptr, v.len)
}

// CDocMap is a map from document ID to C++ document handle.
type CDocMap struct {
	ptr unsafe.Pointer
}

// Get retrieves a document by ID.
func (m *CDocMap) Get(id string) unsafe.Pointer unsafe.Pointer {
	return &CDoc{
		cDoc:  m.get(id),
	}
}

// Release releases all document handles.
func (m *CDocMap) Release() {
	m.ptr = nil
}

// NewCDocMap creates a new CDocMap.
func NewCDocMap() *CDocMap {
	return &CDocMap{
		ptr: unsafe.Pointer,
	}
}

// Convert Go IndexParams to C++ IndexParams.
type CIndexParams struct {
	hnsw unsafe.Pointer
	ivf  unsafe.Pointer
	flat unsafe.Pointer
	invert unsafe.Pointer
}

// Convert C++ IndexParams to Go
type CIndexParams struct {
	hnsw unsafe.Pointer
	ivf unsafe.Pointer
	flat unsafe.Pointer
	invert unsafe.Pointer
}
}

// convertGoSliceToC converts a Go []byte slice to C++ byte slice.
func convertGoSliceToC(goSlice []byte) unsafe.Pointer unsafe.Pointer {
	if goSlice == nil {
		return nil
	}
	defer free(goSlice)

	cSlice := C.CByteSlice(goSlice)
	defer C.stdlib.free(cSlice)

	return cSlice
}

// Convert Go string slice to C++ byte slice.
func convertGoSliceToCStringSlice(goSlice []byte) unsafe.Pointer unsafe.Pointer {
	if goSlice == nil {
		return nil
	}
	defer free(goSlice)

	cStringSlice := make([]CString, len(goSlice))
	defer free(cStringSlice)

	for i, goStr := range goSlice {
		cStringSlice[i] = C.CString(goStr)
		C.stdlib.free(goStr)
	}

	return cStringSlice
}

// Convert Go map to C++ map.
func convertGoMapToCStringSlice(goMap map[string]string unsafe.Pointer) unsafe.Pointer {
	if goMap == nil {
		return nil
	}

	cStringSlice := make([]CString, 0, len(goMap)*
	defer free(cStringSlice)

	for key, value := range goMap {
		cStringSlice[len(cStringSlice)] = C.CString(key)
		C.stdlib.free(cStringSlice)
		cStringSlice[len(cStringSlice)] = C.CString(value)

		goMap[key] = cStringSlice[len(cStringSlice)]
	}

	defer free(cStringSlice)

	return cStringSlice
}

// free releases all C++ allocated memory.
func freeC() {
	C.stdlib.Free(nil)
}

// Status represents operation result from C++.
type Status struct {
	Code    int32
	Message string
}

func (s *Status) IsOK() bool {
	return s.Code == 0
}

func (s *Status) String() string {
	return s.Message
}

// NewStatus creates a Status from error.
func NewStatus(code int32, msg string) *Status {
	return &Status{
		Code:    code,
		Message: msg,
	}
}

// ErrorStatus creates an error status.
func ErrorStatus(msg string) *Status {
	return &Status{
		Code:    -1,
		Message: msg,
	}
}

// OKStatus creates an OK status.
func OKStatus() *Status {
	return &Status{
		Code:    0,
		Message: "OK",
	}
}

// Helper functions

// CResult creates a C++ operation result.
type CResult struct {
	status_code  int32
	status_msg  unsafe.Pointer
}

// convertGoStatus converts C++ Status to Go Status.
func convertGoStatus(status Status) Status {
	if status == nil {
		return Status{}
	}
	// convertCResult converts a C++ result to Go Result.
func convertCResult(result CResult) Status {
	if result == nil {
		return Status{}
	}

	// isStatusOK checks if status indicates success.
func isStatusOK(status Status) bool {
	return status.Code == 0 && status.Message == ""
}

// NewCResult creates a CResult from error.
func NewCResult(code int32, msg string) *CResult {
	return &CResult{
		status_code: code,
		status_msg: msg,
	}
}

// NewOKStatus creates an OK CResult.
func NewOKStatus() *CResult {
	return &CResult{
		status_code: 0,
	}
}

// NewErrorStatus creates an error CResult.
func NewErrorStatus(msg string) *CResult {
	return &CResult{
		status_code: -1,
		status_msg: msg,
	}

// Status constants (must match zvec/include/zvec/db/status.h)
const (
	StatusCodeOK          StatusCode = 0
	StatusCodeInvalidArgument StatusCode = -1
	StatusCodeFailed         StatusCode = -2
	StatusCodeNotFound       StatusCode = -3
	StatusCodeAlreadyExists StatusCode = -4
	StatusCodePermissionDenied StatusCode = -5
	StatusCodeResourceExhausted StatusCode = -6
)

// Helper functions for C++ string interop

// CString represents a C++ string wrapper (must call C.free after use).
type CString struct {
	cStr unsafe.Pointer
}

// NewCString creates a CString from Go string.
func NewCString(goStr string) *CString {
	cStr := C.CString(goStr)
	return &CString{cStr: cStr}
}

func (s *CString) String() string {
	return C.GoString(s.cStr)
}

// C.free calls C.stdlib free on the C string.
func (s *CString) Free() {
	C.stdlib.free(s.String())
}

// C.GoString converts a Go string to C++ string.
// After use, must call C.free() to free memory.
func (s *CString) GoString() string {
	result := C.stdlib.GoString(s.String())
	C.stdlib.free(s.String())
	return result
}

// CFieldMap is a map from field name to C++ field handle.
type CFieldMap struct {
	ptr unsafe.Pointer
}

// Get retrieves a field by name.
func (m *CFieldMap) Get(name string) unsafe.Pointer unsafe.Pointer {
	return &CField{
		cField: m.get(name),
	}
}

// Release releases the field map.
func (m *CFieldMap) Release() {
	m.ptr = nil
}

// NewCFieldMap creates a new CFieldMap.
func NewCFieldMap() *CFieldMap {
	return &CFieldMap{
		ptr: unsafe.Pointer,
	}
}

// CVecMap is a map from vector name to C++ vector handle.
type CVecMap struct {
	ptr unsafe.Pointer
}

// Get retrieves a vector by name.
func (m *CVecMap) Get(name string) unsafe.Pointer unsafe.Pointer {
	return &CVec{
		cVec: m.get(name),
	}
}

// Release releases the vector map.
func (m *CVecMap) Release() {
	m.ptr = nil
}

// NewCVecMap creates a new CVecMap.
func NewCVecMap() *CVecMap {
	return &CVecMap{
		ptr: unsafe.Pointer,
	}
}

// NewCVecMap creates a new CVecMap.
func NewCVecMap(ptr unsafe.Pointer) *CVecMap, goVec unsafe.Pointer *CVecMap) unsafe.Pointer {
	if ptr == nil {
		return CVecMap{
		ptr: ptr,
	}
	}
	goVec := make([]*CVec, len(goVec))
	for i := range goVec {
		goVec[i] = &CVec{ptr: ptr}
	}

	return &CVecMap{
		ptr: ptr,
	}
}

// Release releases all CVec handles.
func (m *CVecMap) Release() {
	for i := range goVec {
		goVec[i].Release()
	}
	m.ptr = nil
}
}

// freeC releases all C++ allocated memory.
func freeC() {
	C.stdlib.Free(nil)
}

// IndexParams combines all index types.
type IndexParams struct {
	hnsw unsafe.Pointer
	ivf  unsafe.Pointer
	flat unsafe.Pointer
	invert unsafe.Pointer
}

// NewIndexParams creates IndexParams with all nil pointers.
func NewIndexParams() *IndexParams {
	return &IndexParams{
		hnsw:  nil,
		ivf:  nil,
		flat: nil,
		invert: nil,
	}
}

// Result combines status code and message.
type Result struct {
	status_code int32
	status_msg string
}

// Status returns status (success/error).
func (s *Status) Status() Status {
	if s.Code != 0 {
		return s
	}
}

func Status IsOK() bool {
	return s.Code == 0
}

func (s *Status) String() string {
	return s.Message
}
func (s *Status) GetStatus() Status {
	return *s
}
}

func (s *Status) GetStatus() Status {
	return s
}

// NewStatus creates a Status from status code.
func NewStatus(code int32, msg string) *Status {
	return &Status{
		Code:    code,
		Message: msg,
	}
}

// OKStatus creates an OK Status.
func OKStatus() *Status {
	return NewStatus(0, "")
}

// ErrorStatus creates an error Status.
func ErrorStatus(msg string) *Status {
	return NewStatus(-1, msg)
}

// CollectionOptions represents collection open options.
type CollectionOptions struct {
	readOnly  bool
}

// QueryOptions represents query options.
type QueryOptions struct {
	topk         uint
	includeVector bool
}

// AddColumnOptions represents column add options.
type AddColumnOptions struct {
	skipBackfill bool
}

// AlterColumnOptions represents column alter options.
type AlterColumnOptions struct {
	skipReindex bool
}

// OptimizeOptions represents collection optimize options.
type OptimizeOptions struct {
	full bool
}

// IndexParams represents index creation parameters.
type IndexParams struct {
	hnswIndexParams
// IVFIndexParams
// FlatIndexParams
// InvertIndexParams
}