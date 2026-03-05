package zvec

import (
	"encoding/json"
	"fmt"
)

// LogType specifies the logger destination.
type LogType string

const (
	LogTypeConsole LogType = "CONSOLE"
	LogTypeFile    LogType = "FILE"
)

// StatusCode represents the status code of an operation.
type StatusCode int

const (
	StatusCodeOK StatusCode = iota
	StatusCodeFailed
	StatusCodeNotFound
	StatusCodeInvalidArgument
	StatusCodeAlreadyExists
	StatusCodePermissionDenied
	StatusCodeResourceExhausted
)

// Status represents the result of an operation.
type Status struct {
	Code    StatusCode `json:"code"`
	Message string     `json:"message,omitempty"`
}

// IsOK returns true if the status is OK.
func (s Status) IsOK() bool {
	return s.Code == StatusCodeOK
}

// Error implements the error interface.
func (s Status) Error() string {
	if s.Code == StatusCodeOK {
		return "OK"
	}
	return fmt.Sprintf("Status(code=%d, message=%q)", s.Code, s.Message)
}

// String returns a string representation of the status.
func (s Status) String() string {
	return s.Error()
}

// LogLevel specifies the minimum log severity.
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// DataType specifies the data type of a field.
type DataType string

const (
	// Scalar data types
	DataTypeInt32     DataType = "INT32"
	DataTypeInt64     DataType = "INT64"
	DataTypeUInt32    DataType = "UINT32"
	DataTypeUInt64    DataType = "UINT64"
	DataTypeFloat     DataType = "FLOAT"
	DataTypeDouble    DataType = "DOUBLE"
	DataTypeString    DataType = "STRING"
	DataTypeBool      DataType = "BOOL"
	DataTypeArrayInt32  DataType = "ARRAY_INT32"
	DataTypeArrayInt64  DataType = "ARRAY_INT64"
	DataTypeArrayUInt32 DataType = "ARRAY_UINT32"
	DataTypeArrayUInt64 DataType = "ARRAY_UINT64"
	DataTypeArrayFloat  DataType = "ARRAY_FLOAT"
	DataTypeArrayDouble DataType = "ARRAY_DOUBLE"
	DataTypeArrayString DataType = "ARRAY_STRING"
	DataTypeArrayBool   DataType = "ARRAY_BOOL"

	// Vector data types
	DataTypeVectorFP16     DataType = "VECTOR_FP16"
	DataTypeVectorFP32     DataType = "VECTOR_FP32"
	DataTypeVectorFP64     DataType = "VECTOR_FP64"
	DataTypeVectorInt8     DataType = "VECTOR_INT8"
	DataTypeSparseVectorFP16 DataType = "SPARSE_VECTOR_FP16"
	DataTypeSparseVectorFP32 DataType = "SPARSE_VECTOR_FP32"
)

// IsScalar checks if the data type is a scalar type.
func (d DataType) IsScalar() bool {
	switch d {
	case DataTypeInt32, DataTypeInt64, DataTypeUInt32, DataTypeUInt64,
		DataTypeFloat, DataTypeDouble, DataTypeString, DataTypeBool,
		DataTypeArrayInt32, DataTypeArrayInt64, DataTypeArrayUInt32,
		DataTypeArrayUInt64, DataTypeArrayFloat, DataTypeArrayDouble,
		DataTypeArrayString, DataTypeArrayBool:
		return true
	default:
		return false
	}
}

// IsVector checks if the data type is a vector type.
func (d DataType) IsVector() bool {
	switch d {
	case DataTypeVectorFP16, DataTypeVectorFP32, DataTypeVectorFP64,
		DataTypeVectorInt8, DataTypeSparseVectorFP16, DataTypeSparseVectorFP32:
		return true
	default:
		return false
	}
}

// State represents the state of a task/node.
type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// MetricType represents the type of metric.
type MetricType string

const (
	MetricTypeL2   MetricType = "L2"
	MetricTypeIP   MetricType = "IP"
	MetricTypeCOSINE MetricType = "COSINE"
)

// String returns the string representation of LogType.
func (l LogType) String() string {
	return string(l)
}

// String returns the string representation of LogLevel.
func (l LogLevel) String() string {
	return string(l)
}

// String returns the string representation of DataType.
func (d DataType) String() string {
	return string(d)
}

// String returns the string representation of State.
func (s State) String() string {
	return string(s)
}

// String returns the string representation of MetricType.
func (m MetricType) String() string {
	return string(m)
}

// MarshalJSON implements json.Marshaler.
func (l LogType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *LogType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*l = LogType(s)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (l LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*l = LogLevel(s)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (d DataType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(d))
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *DataType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*d = DataType(s)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (s State) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *State) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = State(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (m MetricType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(m))
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *MetricType) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = MetricType(v)
	return nil
}
