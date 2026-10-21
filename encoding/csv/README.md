# Typed CSV codec

`github.com/viant/structology/encoding/csv` owns a fixed CSV codec for new consumers,
including Datly v1. The existing SQLX CSV package remains unchanged.

```go
import "github.com/viant/structology/encoding/csv"

type Record struct {
    ID   int    `csvName:"id"`
    Name string `csvName:"name"`
}

encoded, err := csv.Marshal([]Record{{ID: 1, Name: `say "hello", please`}})
var records []Record
err = csv.Unmarshal(encoded, &records)
```

The default writer doubles embedded quotes, preserves commas, backslashes and
newlines inside quoted fields, and writes a header. The reader follows Go's
`encoding/csv` rules, including CRLF normalization. It distinguishes an unquoted
null marker from a quoted literal such as `"null"` when decoding nullable fields.
Zero, false and empty strings remain distinct from null.

`NewMarshaller(reflect.TypeFor[Record](), config)` compiles an immutable typed
shape for repeated concurrent `Marshal` and `Unmarshal` calls. Constructors copy
configuration rather than changing caller-owned settings. The convenience
functions cache default shapes. Decode destinations are nonnil pointers to slices;
decoding appends records and may retain earlier appends if a later row is invalid.

Supported behavior includes struct/pointer/slice/array roots, scalar aliases,
`csvName` aliases, `csvName:"-"` exclusions, hidden presence markers, pointer
scalars, time values and `timeLayout` tags. Nested holders flatten into dotted
headers. Sibling relation slices retain the SQLX Cartesian-row behavior, including
relation-only projections. `UniqueFields` and `References` are rejected explicitly: decoding yields one
root per CSV record and does not infer identity-based regrouping. Recursive shapes and duplicate headers are rejected.

Empty collections and nil roots produce headers without records. Nil elements in
root pointer slices are skipped. Standard delimiters such as comma, semicolon and
tab work for both directions. Explicit nonstandard enclosure/escape settings are
available for writer compatibility; the decoder requires double-quote escaping.
`NewReader` retains the flat streaming adapter: it requires at least one record
and emits headers only when a `StringifierConfig` is supplied.

CSV quoting is not spreadsheet-formula sanitization. Applications exporting
untrusted values for spreadsheets should apply their own cell policy.

## Provenance

Typed relation traversal and scalar formatting were ported from Viant SQLX's
`io/load/reader/csv` and supporting generic helpers under the Apache 2.0 license.
The new codec has no production import of SQLX. Its formatting helper uses Tagly,
consistent with Structology. Changes here do not modify the SQLX implementation.

## Checks

```sh
go test -race ./encoding/csv/...
go test ./encoding/csv -run '^$' -bench BenchmarkMarshal1000 -benchmem
go test ./encoding/csv -run '^$' -fuzz FuzzCSVStrings -fuzztime 5s
```
