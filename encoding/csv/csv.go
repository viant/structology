package csv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	io2 "github.com/viant/structology/encoding/csv/internal/scalar"
	"github.com/viant/xunsafe"
	"io"
	"reflect"
	"time"
)

var timeType = reflect.TypeOf(time.Now())
var timeTypePtr = reflect.PtrTo(timeType)

type (
	Marshaller struct {
		xType           *xunsafe.Type
		elemType        reflect.Type
		xSlice          *xunsafe.Slice
		fieldsPositions map[string]int
		fields          []*Field
		maxDepth        int
		uniquesFields   map[string]bool
		references      map[string][]string
		pathAccessors   map[string]*xunsafe.Field
		pathTypes       map[string]reflect.Type
		stringifiers    map[reflect.Type]*io2.ObjectStringifier
		uniqueTypes     map[reflect.Type]bool
		config          *Config
		compileErr      error
	}

	Field struct {
		parentType reflect.Type
		path       string
		name       string
		header     string
		holder     string

		xField      *xunsafe.Field
		unique      bool
		stringifier io2.FieldStringifierFn
	}

	Reference struct {
		ParentField string
		ChildField  string
	}
)

func NewMarshaller(rType reflect.Type, config *Config) (*Marshaller, error) {
	config = cloneConfig(config)
	if _, err := rowType(rType); err != nil {
		return nil, err
	}

	if config.EncloseBy == "" {
		config.EncloseBy = `"`
	}

	if config.EscapeBy == "" {
		config.EscapeBy = config.EncloseBy
	}

	if config.FieldSeparator == "" {
		config.FieldSeparator = `,`
	}

	if config.ObjectSeparator == "" {
		config.ObjectSeparator = "\n"
	}

	if config.NullValue == "" {
		config.NullValue = "null"
	}

	excluded := map[string]bool{}
	for _, path := range config.ExcludedPaths {
		excluded[path] = true
	}

	elemType := Elem(rType)
	marshaller := &Marshaller{
		config:          config,
		elemType:        elemType,
		fieldsPositions: map[string]int{},
		uniquesFields:   map[string]bool{},
		references:      map[string][]string{},
		pathAccessors:   map[string]*xunsafe.Field{},
		pathTypes:       map[string]reflect.Type{},
		xType:           xunsafe.NewType(elemType),
		uniqueTypes:     map[reflect.Type]bool{},
	}

	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if err := marshaller.init(config, excluded); err != nil {
		return nil, err
	}

	if marshaller.compileErr != nil {
		return nil, marshaller.compileErr
	}
	seen := map[string]bool{}
	for _, field := range marshaller.fields {
		if seen[field.header] {
			return nil, fmt.Errorf("duplicate CSV header %q", field.header)
		}
		seen[field.header] = true
	}
	return marshaller, nil
}

func (m *Marshaller) init(config *Config, excluded map[string]bool) error {
	m.initConfig(config)

	m.xSlice = xunsafe.NewSlice(reflect.SliceOf(m.elemType))
	m.indexByPath(m.elemType, "", excluded, "", nil)

	return nil
}

func (m *Marshaller) indexByPath(parentType reflect.Type, path string, excluded map[string]bool, holder string, parentAccessor *xunsafe.Field) {
	if m.uniqueTypes[parentType] {
		m.compileErr = fmt.Errorf("recursive CSV shape at %s", path)
		return
	}
	m.uniqueTypes[parentType] = true
	defer delete(m.uniqueTypes, parentType)

	elemParentType := Elem(parentType)
	numField := elemParentType.NumField()
	m.pathAccessors[path] = parentAccessor
	m.pathTypes[path] = elemParentType
	for i := 0; i < numField; i++ {
		field := elemParentType.Field(i)
		if field.PkgPath != "" || field.Tag.Get("setMarker") == "true" || field.Tag.Get(TagName) == "-" {
			continue
		}

		fieldPath, fieldName := m.asKeys(path, field)
		if excluded[fieldPath] {
			continue
		}

		elemType := Elem(field.Type)
		if elemType.Kind() == reflect.Struct && elemType != timeType {
			m.indexByPath(elemType, fieldPath, excluded, fieldName, xunsafe.NewField(field))
			continue
		}

		m.fieldsPositions[fieldName] = len(m.fields)
		m.fields = append(m.fields, m.newField(path, holder, field, parentType, fieldPath))
	}
}

func (m *Marshaller) asKeys(path string, field reflect.StructField) (pathKey string, positionsKey string) {
	name := field.Tag.Get(TagName)
	if name != "" {
		key := m.combine(path, name)
		return key, key
	}

	asFullPath := m.combine(path, field.Name)
	return asFullPath, asFullPath
}

func (m *Marshaller) combine(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}

func (m *Marshaller) Unmarshal(b []byte, dest interface{}) error {
	t := reflect.TypeOf(dest)
	if t == nil || t.Kind() != reflect.Pointer || reflect.ValueOf(dest).IsNil() || t.Elem().Kind() != reflect.Slice || Elem(t) != m.elemType {
		return fmt.Errorf("CSV destination must be a nonnil pointer to a slice of %v", m.elemType)
	}
	if (m.config.ObjectSeparator != "\n" && m.config.ObjectSeparator != "\r\n") || m.config.EncloseBy != `"` || m.config.EscapeBy != `"` {
		return fmt.Errorf("CSV decoding requires standard double-quote escaping")
	}

	reader := csv.NewReader(bytes.NewReader(b))
	reader.Comma = []rune(m.config.FieldSeparator)[0]
	headers, err := reader.Read()
	if err != nil {
		return m.asReadError(err)
	}

	fields, err := m.fieldsByName(headers)
	if err != nil {
		return err
	}

	session, err := m.session(fields, dest)
	if err != nil {
		return err
	}

	lineStarts := []int{0}
	for i, char := range b {
		if char == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	for {
		record, err := reader.Read()
		if err != nil {
			return m.asReadError(err)
		}

		if len(record) != len(fields) {
			return fmt.Errorf("record header and the record are differ in length. Fields len: %v, Record len: %v", len(fields), len(record))
		}

		for i, value := range record {
			line, column := reader.FieldPos(i)
			offset := lineStarts[line-1] + column - 1
			quoted := offset < len(b) && b[offset] == '"'
			session.nulls[i] = !quoted && value == m.config.NullValue
		}
		if err = session.addRecord(record); err != nil {
			return err
		}
	}
}

func (m *Marshaller) newField(path string, holder string, field reflect.StructField, parentType reflect.Type, fieldPath string) *Field {
	xField := xunsafe.NewField(field)

	var stringifierConfig *io2.StringifierConfig
	if m.config != nil {
		stringifierConfig = &m.config.StringifierConfig
	}

	return &Field{
		path:        path,
		xField:      xField,
		parentType:  parentType,
		name:        field.Name,
		header:      fieldPath,
		holder:      holder,
		stringifier: io2.Stringifier(xField, false, m.config.NullValue, stringifierConfig),
	}
}

func (m *Marshaller) asReadError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

func (m *Marshaller) initConfig(config *Config) {
	for i := range config.UniqueFields {
		m.uniquesFields[config.UniqueFields[i]] = true
	}

	for _, reference := range config.References {
		m.references[reference.ParentField] = append(m.references[reference.ParentField], reference.ChildField)
	}
}

func (m *Marshaller) session(fields []*Field, dest interface{}) (*UnmarshalSession, error) {
	s := &UnmarshalSession{
		pathIndex: map[string]int{},
		dest:      dest,
	}

	return s, s.init(fields, m.references, m.pathAccessors, m.stringifiers, m.pathTypes)
}

func (m *Marshaller) fieldsByName(names []string) ([]*Field, error) {
	fields := make([]*Field, 0, len(names))
	seen := map[string]bool{}
	for _, header := range names {
		if seen[header] {
			return nil, fmt.Errorf("duplicate CSV input header %q", header)
		}
		seen[header] = true
		index, ok := m.fieldsPositions[header]
		if !ok {
			return nil, fmt.Errorf("not found field %v", header)
		}

		fields = append(fields, m.fields[index])
	}
	return fields, nil
}

func (m *Marshaller) ReadHeaders(b []byte) ([]string, error) {
	reader := csv.NewReader(bytes.NewReader(b))
	reader.Comma = []rune(m.config.FieldSeparator)[0]
	headers, err := reader.Read()
	if err != nil {
		return nil, m.asReadError(err)
	}

	fields, err := m.fieldsByName(headers)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(fields))
	for _, field := range fields {
		result = append(result, m.combine(field.path, field.name))
	}

	return result, nil
}

func (m *Marshaller) Marshal(val interface{}, options ...interface{}) ([]byte, error) {
	valueType, err := rowType(reflect.TypeOf(val))
	if err != nil || valueType != m.elemType {
		return nil, fmt.Errorf("can't marshal %T with %v marshaller", val, m.elemType.String())
	}

	if len(m.fields) == 0 {
		return []byte{}, nil
	}
	values, size, err := io2.Values(val)
	if err != nil {
		return nil, err
	}

	options = append(options, io2.Parallel(true))

	session, err := m.session(m.fields, nil)
	if err != nil {
		return nil, err
	}

	configs := m.marshalOptions(options)
	buffer, err := m.marshalBuffer(values, size, session.parentNode, configs)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(buffer)
}

func (m *Marshaller) marshalOptions(options []interface{}) []*Config {
	var depthConfigs []*Config
	for _, option := range options {
		switch actual := option.(type) {
		case []*Config:
			depthConfigs = actual
		}
	}
	return depthConfigs
}

func (m *Marshaller) marshalBuffer(valueAt io2.ValueAccessor, size int, object *Object, configs []*Config) (*Buffer, error) {
	buffer := NewBuffer(1024)

	accessor, err := object.Accessor(0, m.config, 0, configs)
	if err != nil {
		return nil, err
	}

	newWriter(accessor, m.config, buffer, m.xType, valueAt, size, "").writeObjects(accessor.Headers())

	return buffer, nil
}

func Elem(rType reflect.Type) reflect.Type {
	for {
		switch rType.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array:
			rType = rType.Elem()
		default:
			return rType
		}
	}
}
