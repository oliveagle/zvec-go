// Package cgoz provides a CGO binding to the zvec C++ core
// (the alibaba/zvec submodule under ./zvec) via zvec's official C API
// (zvec/src/include/zvec/c_api.h).
//
// It links against the prebuilt, self-contained C API shared libraries
// checked in under ../lib, one per platform subdirectory:
//
//	lib/linux-x86_64/libzvec_c_api.so
//	lib/linux-arm64/libzvec_c_api.so
//	lib/macos-arm64/libzvec_c_api.dylib
//
// Those libraries embed the entire C++ core plus all third-party
// dependencies, so a binary built here has no runtime dependencies beyond
// system libraries (libc/libm/libpthread/libdl). The FTS tokenizer needs
// the data files under ../lib/data/jieba_dict at runtime; point
// ZVEC_JIEBA_DICT_DIR (or the collection's FTS config) at that directory
// when using FTS.
//
// This binding is OPTIONAL and is not required by the pure-Go client (the
// root "zvec" package) or the HTTP service (./server, ./cmd/zvec-httpd).
// To build against the real C++ core, enable the build tags:
//
//	CGO_ENABLED=1 go build -tags cgo,zvec_cgo ./cgo/
//
// The prebuilt libraries in ../lib must match the version the zvec submodule
// is pinned to (currently v0.7.0 — see ../lib/README.md). When the submodule
// moves to a new version, re-download the matching prebuilt SDK from the
// alibaba/zvec GitHub release and replace the libraries in ../lib.
//
//go:build cgo && zvec_cgo

package cgoz

/*
#cgo CFLAGS: -I${SRCDIR}/../zvec/src/include
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../lib/linux-x86_64/libzvec_c_api.so -Wl,-rpath,${SRCDIR}/../lib/linux-x86_64
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/../lib/linux-arm64/libzvec_c_api.so -Wl,-rpath,${SRCDIR}/../lib/linux-arm64
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../lib/macos-arm64/libzvec_c_api.dylib -Wl,-rpath,${SRCDIR}/../lib/macos-arm64
#include <zvec/c_api.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"fmt"
	"regexp"
	"runtime"
	"unsafe"
)

// Version identifies this binding.
const Version = "cgo-c-api"

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

// ErrorCode mirrors zvec_error_code_t from the C API.
type ErrorCode C.zvec_error_code_t

const (
	ErrOK                ErrorCode = C.ZVEC_OK
	ErrNotFound          ErrorCode = C.ZVEC_ERROR_NOT_FOUND
	ErrAlreadyExists     ErrorCode = C.ZVEC_ERROR_ALREADY_EXISTS
	ErrInvalidArgument   ErrorCode = C.ZVEC_ERROR_INVALID_ARGUMENT
	ErrPermissionDenied  ErrorCode = C.ZVEC_ERROR_PERMISSION_DENIED
	ErrFailedPrecond     ErrorCode = C.ZVEC_ERROR_FAILED_PRECONDITION
	ErrResourceExhausted ErrorCode = C.ZVEC_ERROR_RESOURCE_EXHAUSTED
	ErrUnavailable       ErrorCode = C.ZVEC_ERROR_UNAVAILABLE
	ErrInternal          ErrorCode = C.ZVEC_ERROR_INTERNAL_ERROR
	ErrNotSupported      ErrorCode = C.ZVEC_ERROR_NOT_SUPPORTED
	ErrUnknown           ErrorCode = C.ZVEC_ERROR_UNKNOWN
)

func (c ErrorCode) String() string {
	switch c {
	case ErrOK:
		return "ok"
	case ErrNotFound:
		return "not found"
	case ErrAlreadyExists:
		return "already exists"
	case ErrInvalidArgument:
		return "invalid argument"
	case ErrPermissionDenied:
		return "permission denied"
	case ErrFailedPrecond:
		return "failed precondition"
	case ErrResourceExhausted:
		return "resource exhausted"
	case ErrUnavailable:
		return "unavailable"
	case ErrInternal:
		return "internal error"
	case ErrNotSupported:
		return "not supported"
	default:
		return fmt.Sprintf("unknown(%d)", int(c))
	}
}

// lastError returns a Go error for a non-OK C status code, enriching it with
// the library's last error message.
func lastError(code C.zvec_error_code_t) error {
	if code == C.ZVEC_OK {
		return nil
	}
	var msg **C.char
	ec := C.zvec_get_last_error(msg)
	var detail string
	if C.zvec_error_code_t(ec) == C.ZVEC_OK && msg != nil && *msg != nil {
		detail = C.GoString(*msg)
		C.free(unsafe.Pointer(*msg))
	}
	C.zvec_clear_error()
	if detail == "" {
		return fmt.Errorf("zvec: %s", ErrorCode(code).String())
	}
	return fmt.Errorf("zvec: %s: %s", ErrorCode(code).String(), detail)
}

func check(code C.zvec_error_code_t, op string) error {
	if code == C.ZVEC_OK {
		return nil
	}
	return fmt.Errorf("zvec: %s: %w", op, lastError(code))
}

// ---------------------------------------------------------------------------
// Version / library info
// ---------------------------------------------------------------------------

// VersionString returns the zvec C++ core version string reported by the
// linked library, e.g. "v0.7.0".
func VersionString() string {
	return C.GoString(C.zvec_get_version())
}

// CheckVersion reports whether the linked zvec core is at least
// major.minor.patch (semantic versioning).
func CheckVersion(major, minor, patch int) bool {
	return bool(C.zvec_check_version(C.int(major), C.int(minor), C.int(patch)))
}

// ---------------------------------------------------------------------------
// Metrics / index types
// ---------------------------------------------------------------------------

// Metric is a distance metric for vector indexes (mirrors zvec_metric_type_t).
type Metric C.zvec_metric_type_t

const (
	MetricL2     Metric = C.ZVEC_METRIC_TYPE_L2
	MetricIP     Metric = C.ZVEC_METRIC_TYPE_IP
	MetricCosine Metric = C.ZVEC_METRIC_TYPE_COSINE
)

// IndexType is a vector/scalar index type (mirrors zvec_index_type_t).
type IndexType C.zvec_index_type_t

const (
	IndexHNSW IndexType = C.ZVEC_INDEX_TYPE_HNSW
	IndexIVF  IndexType = C.ZVEC_INDEX_TYPE_IVF
	IndexFlat IndexType = C.ZVEC_INDEX_TYPE_FLAT
)

// HnswParams configures an HNSW index.
type HnswParams struct {
	M              int
	EfConstruction int
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

// Schema builds a zvec collection schema (mirrors zvec_collection_schema_t).
// A schema must be Free()d (or its finalizer runs) when no longer needed.
type Schema struct {
	ptr *C.zvec_collection_schema_t
}

// collectionNameRe and fieldNameRe mirror the naming rules enforced by the
// zvec C++ core (zvec/src/db/common/constants.h).
var (
	collectionNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
	fieldNameRe      = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
)

// NewSchema creates a collection schema with the given name. The name must
// match ^[a-zA-Z0-9_-]{3,64}$, the zvec core's naming rule.
func NewSchema(name string) (*Schema, error) {
	if !collectionNameRe.MatchString(name) {
		return nil, fmt.Errorf("zvec: invalid collection name %q: must match ^[a-zA-Z0-9_-]{3,64}$", name)
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	p := C.zvec_collection_schema_create(cname)
	if p == nil {
		return nil, fmt.Errorf("zvec: create schema %q: %w", name, lastError(C.ZVEC_ERROR_INVALID_ARGUMENT))
	}
	s := &Schema{ptr: p}
	runtime.SetFinalizer(s, (*Schema).Free)
	return s, nil
}

// AddVectorField adds a FP32 vector field with an index of the given type,
// metric and dimension. hnsw may be nil to use library defaults.
func (s *Schema) AddVectorField(name string, dim int, index IndexType, metric Metric, hnsw *HnswParams) error {
	if s.ptr == nil {
		return fmt.Errorf("zvec: schema already freed")
	}
	if !fieldNameRe.MatchString(name) {
		return fmt.Errorf("zvec: invalid field name %q: must match ^[a-zA-Z0-9_-]{1,32}$", name)
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	var field *C.zvec_field_schema_t
	field = C.zvec_field_schema_create(cname, C.ZVEC_DATA_TYPE_VECTOR_FP32, C.bool(false), C.uint32_t(dim))
	if field == nil {
		return fmt.Errorf("zvec: create field %q: %w", name, lastError(C.ZVEC_ERROR_INVALID_ARGUMENT))
	}
	defer C.zvec_field_schema_destroy(field)

	var idx *C.zvec_index_params_t
	idx = C.zvec_index_params_create(C.zvec_index_type_t(index))
	if idx == nil {
		return fmt.Errorf("zvec: create index params for %q: %w", name, lastError(C.ZVEC_ERROR_INVALID_ARGUMENT))
	}
	defer C.zvec_index_params_destroy(idx)
	if err := check(C.zvec_index_params_set_metric_type(idx, C.zvec_metric_type_t(metric)), "set metric"); err != nil {
		return err
	}
	if index == IndexHNSW {
		m, efc := 16, 200
		if hnsw != nil {
			if hnsw.M > 0 {
				m = hnsw.M
			}
			if hnsw.EfConstruction > 0 {
				efc = hnsw.EfConstruction
			}
		}
		if err := check(C.zvec_index_params_set_hnsw_params(idx, C.int(m), C.int(efc)), "set hnsw params"); err != nil {
			return err
		}
	}
	if err := check(C.zvec_field_schema_set_index_params(field, idx), "set index params"); err != nil {
		return err
	}
	return check(C.zvec_collection_schema_add_field(s.ptr, field), "add field "+name)
}

// AddStringField adds a string field.
func (s *Schema) AddStringField(name string, nullable bool) error {
	return s.addScalarField(name, C.ZVEC_DATA_TYPE_STRING, nullable)
}

// AddInt64Field adds an INT64 field.
func (s *Schema) AddInt64Field(name string, nullable bool) error {
	return s.addScalarField(name, C.ZVEC_DATA_TYPE_INT64, nullable)
}

// AddInt32Field adds an INT32 field.
func (s *Schema) AddInt32Field(name string, nullable bool) error {
	return s.addScalarField(name, C.ZVEC_DATA_TYPE_INT32, nullable)
}

// AddFloat64Field adds a DOUBLE field.
func (s *Schema) AddFloat64Field(name string, nullable bool) error {
	return s.addScalarField(name, C.ZVEC_DATA_TYPE_DOUBLE, nullable)
}

func (s *Schema) addScalarField(name string, dt C.zvec_data_type_t, nullable bool) error {
	if s.ptr == nil {
		return fmt.Errorf("zvec: schema already freed")
	}
	if !fieldNameRe.MatchString(name) {
		return fmt.Errorf("zvec: invalid field name %q: must match ^[a-zA-Z0-9_-]{1,32}$", name)
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	field := C.zvec_field_schema_create(cname, dt, C.bool(nullable), 0)
	if field == nil {
		return fmt.Errorf("zvec: create field %q: %w", name, lastError(C.ZVEC_ERROR_INVALID_ARGUMENT))
	}
	defer C.zvec_field_schema_destroy(field)
	return check(C.zvec_collection_schema_add_field(s.ptr, field), "add field "+name)
}

// Free releases the native schema. Safe to call more than once.
func (s *Schema) Free() {
	if s.ptr != nil {
		C.zvec_collection_schema_destroy(s.ptr)
		s.ptr = nil
	}
	runtime.SetFinalizer(s, nil)
}

// ---------------------------------------------------------------------------
// Document
// ---------------------------------------------------------------------------

// Document is a mutable zvec document (mirrors zvec_doc_t). Free it when done
// (or let the finalizer run).
type Document struct {
	ptr *C.zvec_doc_t
}

// NewDocument creates an empty document with the given primary key.
func NewDocument(pk string) (*Document, error) {
	p := C.zvec_doc_create()
	if p == nil {
		return nil, fmt.Errorf("zvec: create doc: %w", lastError(C.ZVEC_ERROR_INTERNAL_ERROR))
	}
	cpk := C.CString(pk)
	defer C.free(unsafe.Pointer(cpk))
	C.zvec_doc_set_pk(p, cpk)
	d := &Document{ptr: p}
	runtime.SetFinalizer(d, (*Document).Free)
	return d, nil
}

func (d *Document) check() error {
	if d.ptr == nil {
		return fmt.Errorf("zvec: document already freed")
	}
	return nil
}

// SetString sets a string field value.
func (d *Document) SetString(name string, value string) error {
	if err := d.check(); err != nil {
		return err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	cv := C.CString(value)
	defer C.free(unsafe.Pointer(cv))
	return check(C.zvec_doc_add_field_by_value(d.ptr, cn, C.ZVEC_DATA_TYPE_STRING, unsafe.Pointer(cv), C.size_t(len(value)+1)), "set field "+name)
}

// SetInt64 sets an INT64 field value.
func (d *Document) SetInt64(name string, value int64) error {
	if err := d.check(); err != nil {
		return err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	return check(C.zvec_doc_add_field_by_value(d.ptr, cn, C.ZVEC_DATA_TYPE_INT64, unsafe.Pointer(&value), C.size_t(8)), "set field "+name)
}

// SetInt32 sets an INT32 field value.
func (d *Document) SetInt32(name string, value int32) error {
	if err := d.check(); err != nil {
		return err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	return check(C.zvec_doc_add_field_by_value(d.ptr, cn, C.ZVEC_DATA_TYPE_INT32, unsafe.Pointer(&value), C.size_t(4)), "set field "+name)
}

// SetFloat64 sets a DOUBLE field value.
func (d *Document) SetFloat64(name string, value float64) error {
	if err := d.check(); err != nil {
		return err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	return check(C.zvec_doc_add_field_by_value(d.ptr, cn, C.ZVEC_DATA_TYPE_DOUBLE, unsafe.Pointer(&value), C.size_t(8)), "set field "+name)
}

// SetVectorFP32 sets an FP32 vector field value.
func (d *Document) SetVectorFP32(name string, vec []float32) error {
	if err := d.check(); err != nil {
		return err
	}
	if len(vec) == 0 {
		return fmt.Errorf("zvec: empty vector for field %q", name)
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	return check(C.zvec_doc_add_field_by_value(d.ptr, cn, C.ZVEC_DATA_TYPE_VECTOR_FP32, unsafe.Pointer(&vec[0]), C.size_t(len(vec)*4)), "set vector "+name)
}

// PK returns the document primary key (a Go string copy; safe after Free).
func (d *Document) PK() string {
	if err := d.check(); err != nil {
		return ""
	}
	cp := C.zvec_doc_get_pk_copy(d.ptr)
	if cp == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cp))
	return C.GoString(cp)
}

// Score returns the last search score assigned to the document.
func (d *Document) Score() float64 {
	if err := d.check(); err != nil {
		return 0
	}
	return float64(C.zvec_doc_get_score(d.ptr))
}

// HasField reports whether the document contains the field.
func (d *Document) HasField(name string) bool {
	if err := d.check(); err != nil {
		return false
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	return bool(C.zvec_doc_has_field(d.ptr, cn))
}

// GetString reads a string field value (Go string copy).
func (d *Document) GetString(name string) (string, error) {
	if err := d.check(); err != nil {
		return "", err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	var vp unsafe.Pointer
	var sz C.size_t
	if err := check(C.zvec_doc_get_field_value_pointer(d.ptr, cn, C.ZVEC_DATA_TYPE_STRING, &vp, &sz), "get field "+name); err != nil {
		return "", err
	}
	if vp == nil {
		return "", nil
	}
	return C.GoString((*C.char)(vp)), nil
}

// GetInt64 reads an INT64 field value.
func (d *Document) GetInt64(name string) (int64, error) {
	if err := d.check(); err != nil {
		return 0, err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	var v int64
	if err := check(C.zvec_doc_get_field_value_basic(d.ptr, cn, C.ZVEC_DATA_TYPE_INT64, unsafe.Pointer(&v), C.size_t(8)), "get field "+name); err != nil {
		return 0, err
	}
	return v, nil
}

// GetVectorFP32 reads an FP32 vector field value (Go slice copy).
func (d *Document) GetVectorFP32(name string) ([]float32, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	var vp unsafe.Pointer
	var sz C.size_t
	if err := check(C.zvec_doc_get_field_value_pointer(d.ptr, cn, C.ZVEC_DATA_TYPE_VECTOR_FP32, &vp, &sz), "get vector "+name); err != nil {
		return nil, err
	}
	if vp == nil || sz == 0 {
		return nil, nil
	}
	n := int(sz / 4)
	out := make([]float32, n)
	copy(out, unsafe.Slice((*float32)(vp), n))
	return out, nil
}

// Free releases the native document. Safe to call more than once.
func (d *Document) Free() {
	if d.ptr != nil {
		C.zvec_doc_destroy(d.ptr)
		d.ptr = nil
	}
	runtime.SetFinalizer(d, nil)
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

// Query is a vector search request (mirrors zvec_vector_query_t).
type Query struct {
	ptr *C.zvec_vector_query_t
}

// NewQuery creates a vector query searching field with vector, returning up
// to topk results (topk <= 0 uses the library default of 10).
func NewQuery(field string, vec []float32, topk int) (*Query, error) {
	if len(vec) == 0 {
		return nil, fmt.Errorf("zvec: empty query vector")
	}
	p := C.zvec_vector_query_create()
	if p == nil {
		return nil, fmt.Errorf("zvec: create query: %w", lastError(C.ZVEC_ERROR_INTERNAL_ERROR))
	}
	cf := C.CString(field)
	defer C.free(unsafe.Pointer(cf))
	if err := check(C.zvec_vector_query_set_field_name(p, cf), "set query field"); err != nil {
		C.zvec_vector_query_destroy(p)
		return nil, err
	}
	if err := check(C.zvec_vector_query_set_query_vector(p, unsafe.Pointer(&vec[0]), C.size_t(len(vec)*4)), "set query vector"); err != nil {
		C.zvec_vector_query_destroy(p)
		return nil, err
	}
	if topk > 0 {
		if err := check(C.zvec_vector_query_set_topk(p, C.int(topk)), "set topk"); err != nil {
			C.zvec_vector_query_destroy(p)
			return nil, err
		}
	}
	q := &Query{ptr: p}
	runtime.SetFinalizer(q, (*Query).Free)
	return q, nil
}

// Free releases the native query. Safe to call more than once.
func (q *Query) Free() {
	if q.ptr != nil {
		C.zvec_vector_query_destroy(q.ptr)
		q.ptr = nil
	}
	runtime.SetFinalizer(q, nil)
}

// ---------------------------------------------------------------------------
// Collection
// ---------------------------------------------------------------------------

// CollectionOptions controls how a collection is opened.
type CollectionOptions struct {
	ReadOnly    bool
	EnableMmap  bool
	MaxBufferKB int
}

// Collection is an open zvec collection (mirrors zvec_collection_t).
type Collection struct {
	ptr *C.zvec_collection_t
}

func collectionOptions(o *CollectionOptions) (*C.zvec_collection_options_t, func()) {
	if o == nil {
		return nil, func() {}
	}
	p := C.zvec_collection_options_create()
	if p == nil {
		return nil, func() {}
	}
	C.zvec_collection_options_set_read_only(p, C.bool(o.ReadOnly))
	C.zvec_collection_options_set_enable_mmap(p, C.bool(o.EnableMmap))
	if o.MaxBufferKB > 0 {
		C.zvec_collection_options_set_max_buffer_size(p, C.size_t(o.MaxBufferKB*1024))
	}
	return p, func() { C.zvec_collection_options_destroy(p) }
}

// CreateAndOpen creates a new collection at path with the given schema and
// opens it.
func CreateAndOpen(path string, schema *Schema, opts *CollectionOptions) (*Collection, error) {
	if schema == nil || schema.ptr == nil {
		return nil, fmt.Errorf("zvec: schema is required")
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	optsC, freeOpts := collectionOptions(opts)
	defer freeOpts()
	var pc *C.zvec_collection_t
	ec := C.zvec_collection_create_and_open(cpath, schema.ptr, optsC, &pc)
	if ec != C.ZVEC_OK {
		return nil, fmt.Errorf("zvec: create and open %q: %w", path, lastError(ec))
	}
	c := &Collection{ptr: pc}
	runtime.SetFinalizer(c, (*Collection).Close)
	return c, nil
}

// Open opens an existing collection at path.
func Open(path string, opts *CollectionOptions) (*Collection, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	optsC, freeOpts := collectionOptions(opts)
	defer freeOpts()
	var pc *C.zvec_collection_t
	ec := C.zvec_collection_open(cpath, optsC, &pc)
	if ec != C.ZVEC_OK {
		return nil, fmt.Errorf("zvec: open %q: %w", path, lastError(ec))
	}
	c := &Collection{ptr: pc}
	runtime.SetFinalizer(c, (*Collection).Close)
	return c, nil
}

func (c *Collection) check() error {
	if c.ptr == nil {
		return fmt.Errorf("zvec: collection already closed")
	}
	return nil
}

func (c *Collection) writeDocs(op string, docs []*Document) (written, failed int, err error) {
	if err := c.check(); err != nil {
		return 0, 0, err
	}
	if len(docs) == 0 {
		return 0, 0, nil
	}
	ptrs := make([]*C.zvec_doc_t, len(docs))
	for i, d := range docs {
		if d.ptr == nil {
			return 0, 0, fmt.Errorf("zvec: doc[%d] already freed", i)
		}
		ptrs[i] = d.ptr
	}
	var ok, fail C.size_t
	var ec C.zvec_error_code_t
	switch op {
	case "insert":
		ec = C.zvec_collection_insert(c.ptr, &ptrs[0], C.size_t(len(docs)), &ok, &fail)
	case "update":
		ec = C.zvec_collection_update(c.ptr, &ptrs[0], C.size_t(len(docs)), &ok, &fail)
	case "upsert":
		ec = C.zvec_collection_upsert(c.ptr, &ptrs[0], C.size_t(len(docs)), &ok, &fail)
	default:
		return 0, 0, fmt.Errorf("zvec: unknown op %q", op)
	}
	if ec != C.ZVEC_OK {
		return int(ok), int(fail), fmt.Errorf("zvec: %s: %w", op, lastError(ec))
	}
	return int(ok), int(fail), nil
}

// Insert inserts the given documents.
func (c *Collection) Insert(docs ...*Document) (written, failed int, err error) {
	return c.writeDocs("insert", docs)
}

// Update updates the given documents.
func (c *Collection) Update(docs ...*Document) (written, failed int, err error) {
	return c.writeDocs("update", docs)
}

// Upsert inserts new or updates existing documents by primary key.
func (c *Collection) Upsert(docs ...*Document) (written, failed int, err error) {
	return c.writeDocs("upsert", docs)
}

// Delete removes documents by primary key.
func (c *Collection) Delete(ids ...string) (removed, failed int, err error) {
	if err := c.check(); err != nil {
		return 0, 0, err
	}
	if len(ids) == 0 {
		return 0, 0, nil
	}
	cids := make([]*C.char, len(ids))
	for i, id := range ids {
		cids[i] = C.CString(id)
	}
	defer func() {
		for _, ci := range cids {
			C.free(unsafe.Pointer(ci))
		}
	}()
	var ok, fail C.size_t
	ec := C.zvec_collection_delete(c.ptr, &cids[0], C.size_t(len(ids)), &ok, &fail)
	if ec != C.ZVEC_OK {
		return int(ok), int(fail), fmt.Errorf("zvec: delete: %w", lastError(ec))
	}
	return int(ok), int(fail), nil
}

// Query runs a vector similarity search and returns the result documents.
// The returned *Document values own their native documents; Free them (or let
// their finalizers run) when done.
func (c *Collection) Query(q *Query) (docs []*Document, err error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if q.ptr == nil {
		return nil, fmt.Errorf("zvec: query already freed")
	}
	var results **C.zvec_doc_t
	var count C.size_t
	ec := C.zvec_collection_query(c.ptr, q.ptr, &results, &count)
	if ec != C.ZVEC_OK {
		return nil, fmt.Errorf("zvec: query: %w", lastError(ec))
	}
	if count == 0 || results == nil {
		return nil, nil
	}
	// The C call returns a native block of doc pointers. Copy the pointer
	// values into Go-owned wrappers FIRST, then release only the native
	// array block. Each *Document owns (and frees) its own native doc, so
	// the docs outlive this call — zvec_docs_free must NOT be deferred here
	// (it would destroy the docs before the caller can use them).
	block := (*[1 << 30]*C.zvec_doc_t)(unsafe.Pointer(results))
	docsC := block[:count]
	docs = make([]*Document, int(count))
	for i := range docsC {
		docs[i] = &Document{ptr: docsC[i]}
		runtime.SetFinalizer(docs[i], (*Document).Free)
	}
	C.free(unsafe.Pointer(results))
	return docs, nil
}

// DocCount returns the number of documents in the collection, if exposed by
// the linked version.
func (c *Collection) DocCount() (int, error) {
	if err := c.check(); err != nil {
		return 0, err
	}
	// The C API exposes statistics through an opaque stats handle; this
	// binding keeps the surface minimal and reports -1 when unavailable.
	return -1, nil
}

// Close releases the handle. The underlying collection persists on disk and
// is closed when the last handle is released.
func (c *Collection) Close() error {
	if c.ptr != nil {
		C.zvec_collection_close(c.ptr)
		c.ptr = nil
	}
	runtime.SetFinalizer(c, nil)
	return nil
}

// Destroy closes the collection and permanently deletes its data on disk.
func (c *Collection) Destroy() error {
	if err := c.check(); err != nil {
		return err
	}
	C.zvec_collection_destroy(c.ptr)
	c.ptr = nil
	runtime.SetFinalizer(c, nil)
	return nil
}
