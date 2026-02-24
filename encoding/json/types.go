package json

import (
	"context"
	"unsafe"

	"github.com/viant/tagly/format"
	"github.com/viant/tagly/format/text"
)

// Mode controls compatibility vs strict behavior.
type Mode int

const (
	ModeCompat Mode = iota
	ModeStrict
)

// UnknownFieldPolicy controls unknown key handling.
type UnknownFieldPolicy int

const (
	IgnoreUnknown UnknownFieldPolicy = iota
	ErrorOnUnknown
)

// NumberPolicy controls numeric coercion behavior.
type NumberPolicy int

const (
	CoerceNumbers NumberPolicy = iota
	ExactNumbers
)

// NullPolicy controls null assignment behavior.
type NullPolicy int

const (
	CompatNulls NullPolicy = iota
	StrictNulls
)

// DuplicateKeyPolicy controls duplicate object key behavior.
type DuplicateKeyPolicy int

const (
	LastWins DuplicateKeyPolicy = iota
	ErrorOnDuplicate
)

// MalformedPolicy controls malformed JSON tolerance.
type MalformedPolicy int

const (
	Tolerant MalformedPolicy = iota
	FailFast
)

// PathTrackingMode controls path tracking overhead.
type PathTrackingMode int

const (
	PathTrackingOff PathTrackingMode = iota
	PathTrackingErrorsOnly
	PathTrackingFull
)

// NilSlicePolicy controls marshal output for nil slices.
type NilSlicePolicy int

const (
	NilSliceAsNull NilSlicePolicy = iota
	NilSliceAsEmptyArray
)

// NameTransformer transforms field names for output and path display.
type NameTransformer interface {
	Transform(path, fieldName string) string
}

// FieldExcluder decides whether a field should be excluded.
type FieldExcluder interface {
	Exclude(path, fieldName string) bool
}

// PathNameTransformer transforms names using path segments without forcing path string joins.
type PathNameTransformer interface {
	TransformPath(path []string, fieldName string) string
}

// PathFieldExcluder excludes fields using path segments without forcing path string joins.
type PathFieldExcluder interface {
	ExcludePath(path []string, fieldName string) bool
}

// Decoder defines decode-side methods used by generic unmarshalers.
type Decoder interface {
	String(*string) error
	Int(*int) error
	Int8(*int8) error
	Int16(*int16) error
	Int32(*int32) error
	Int64(*int64) error
	Uint(*uint) error
	Uint8(*uint8) error
	Uint16(*uint16) error
	Uint32(*uint32) error
	Uint64(*uint64) error
	Float32(*float32) error
	Float64(*float64) error
	Bool(*bool) error
	Interface(*interface{}) error
	Object(UnmarshalerJSONObject) error
	Array(UnmarshalerJSONArray) error
}

// PathRef references path segments without eager string allocation.
type PathRef struct {
	segments []PathSegment
	depth    int
}

func (p PathRef) Len() int { return p.depth }

func (p PathRef) At(i int) (PathSegment, bool) {
	if i < 0 || i >= p.depth {
		return PathSegment{}, false
	}
	return p.segments[i], true
}

func (p PathRef) Segments() []PathSegment {
	if p.depth == 0 {
		return nil
	}
	out := make([]PathSegment, p.depth)
	copy(out, p.segments[:p.depth])
	return out
}

// PathSegment describes one path component.
type PathSegment struct {
	Field string
	Index int
	Kind  SegmentKind
}

// SegmentKind identifies path segment type.
type SegmentKind int

const (
	SegmentField SegmentKind = iota
	SegmentIndex
)

// Option mutates runtime options.
type Option interface{ apply(*Options) }

// Options defines runtime behavior.
type Options struct {
	Ctx context.Context

	Mode               Mode
	UnknownFieldPolicy UnknownFieldPolicy
	NumberPolicy       NumberPolicy
	NullPolicy         NullPolicy
	DuplicateKeyPolicy DuplicateKeyPolicy
	MalformedPolicy    MalformedPolicy
	PathTracking       PathTrackingMode

	CaseFormat         text.CaseFormat
	FormatTag          *format.Tag
	TimeLayout         string
	NameTransformer    NameTransformer
	FieldExcluder      FieldExcluder
	PathName           PathNameTransformer
	PathExcluder       PathFieldExcluder
	DebugPathSink      func(PathRef)
	FieldUnmarshalHook func(ctx context.Context, holder unsafe.Pointer, field string, value any) (any, error)
	PathUnmarshalHook  func(ctx context.Context, holder unsafe.Pointer, path []string, field string, value any) (any, error)
	scannerHooks       ScannerHooks
	OmitEmpty          bool
	NilSlicePolicy     NilSlicePolicy

	setMode               bool
	setUnknownFieldPolicy bool
	setNumberPolicy       bool
	setNullPolicy         bool
	setDuplicateKeyPolicy bool
	setMalformedPolicy    bool
	setPathTracking       bool
	setCaseFormat         bool
}

// Codec is a unified encoder/decoder contract.
type Codec interface {
	String(*string) error
	Int(*int) error
	Int8(*int8) error
	Int16(*int16) error
	Int32(*int32) error
	Int64(*int64) error
	Uint(*uint) error
	Uint8(*uint8) error
	Uint16(*uint16) error
	Uint32(*uint32) error
	Uint64(*uint64) error
	Float32(*float32) error
	Float64(*float64) error
	Bool(*bool) error
	Interface(*interface{}) error
	Object(UnmarshalerObject) error
	Array(UnmarshalerArray) error

	AddString(string)
	AddInt(int)
	AddInt8(int8)
	AddInt16(int16)
	AddInt32(int32)
	AddInt64(int64)
	AddUint(uint)
	AddUint8(uint8)
	AddUint16(uint16)
	AddUint32(uint32)
	AddUint64(uint64)
	AddFloat32(float32)
	AddFloat64(float64)
	AddBool(bool)
	AddNull()
	AddInterface(interface{}) error
	AddObject(MarshalerObject)
	AddArray(MarshalerArray)
}

// UnmarshalerObject receives object fields.
type UnmarshalerJSONObject interface {
	UnmarshalJSONObject(context.Context, Decoder, string) error
	NKeys() int
}

// UnmarshalerJSONArray receives array elements.
type UnmarshalerJSONArray interface {
	UnmarshalJSONArray(context.Context, Decoder) error
}

// UnmarshalerObject is backward-compatible alias style interface.
type UnmarshalerObject interface {
	UnmarshalObject(context.Context, Codec, string) error
	NKeys() int
}

// UnmarshalerArray receives array elements.
type UnmarshalerArray interface {
	UnmarshalArray(context.Context, Codec) error
}

// MarshalerObject emits object fields.
type MarshalerObject interface {
	MarshalObject(context.Context, Codec)
	IsNil() bool
}

// MarshalerArray emits array elements.
type MarshalerArray interface {
	MarshalArray(context.Context, Codec)
	IsNil() bool
}
