# structology - State tracker for golang struct 
[![GoReportCard](https://goreportcard.com/badge/github.com/viant/structology)](https://goreportcard.com/report/github.com/viant/structology)
[![GoDoc](https://godoc.org/github.com/viant/structology?status.svg)](https://godoc.org/github.com/viant/structology)

This library is compatible with Go 1.23+

- [Motivation](#motivation)
- [Usage](#usage)
- [Why Another JSON Engine](#why-another-json-engine)
- [JSON Benchmarks](#json-benchmarks)
- [Contribution](#contributing-to-structology)
- [License](#license)

## Motivation


This project defines struct field set marker to distinctively identify input state, specifically for 'empty', 'nil', 'has been set' state.
This is critical for update/patch operation with large struct where only small subset if defined, 
with presence marker allowing you handling user input effectively, ensuring data integrity, and improving the security of applications  

Initially we have a few project holding marker abstraction to finally move it to this project.  

This project also implement a state that wraps arbitrary go struct, and provide selectors
to dynamically access/mutate any nested/addressable values, if set marker is defined all set operation also
would flag respective marker field 

Why markers vs alternatives

- Pointers/nullables: encode absence but pollute type shape and struggle with zero vs unset for primitives.
- Zero-value heuristics: cannot distinguish deliberate zero from "unset", causing accidental overwrites.
- Patch DTOs: explicit but require parallel types and maintenance-heavy mapping.
- Field masks: powerful yet add separate infrastructure and don’t fit plain Go structs naturally.

Markers keep models simple, make intent explicit, and integrate with path selectors for ergonomic partial updates.

## Usage

##### Set Marker
```go
type (
    HasEntity struct {
	    Id     bool
        Name   bool
        Active bool
	}
     Entity struct {
        Id     int
        Name   string
        Active bool
        Has    *EntityHas `setMarker:"true"`
    }
)

    var entity *Entity

	// load entity and set all present fields with marker-aware setters

	// Build a marker for Entity by type
	m, err := structology.NewMarker(reflect.TypeOf(&Entity{}))
	if err != nil {
		log.Fatal(err)
	}
	ptr := xunsafe.AsPointer(entity)
	isIdSet := m.IsSet(ptr, m.Index("Id"))
	fmt.Printf("is ID set: %v\n", isIdSet)
	
}
```

##### State with SetMarker

```go
package bar

import (
	"fmt"
	"reflect"
	"github.com/viant/structology"
	"encoding/json"
)

type FooHas struct {
	Id   bool
	Name bool
}

type DummyHas struct {
	Id  bool
	Foo bool
}

type Foo struct {
	Id   int
	Name string
	Has  *FooHas `setMarker:"true"`
}

type Dummy struct {
	Id  int
	Foo *Foo
	Has *DummyHas `setMarker:"true"`
}

func ShowStateUsage() {
	
	dummy := &Dummy{Foo: &Foo{}}
	valueType := reflect.TypeOf(dummy)
	stateType := structology.NewStateType(valueType)
	state := stateType.WithValue(dummy)
	
	hasFoo, _ := state.Bool("Has.Foo")
	fmt.Printf("initial foo has marker: %v\n", 	hasFoo)
	hasFooName, _ := 	state.Bool("Foo.Has.Name") 
	fmt.Printf("initial foo has marker: %v\n", 	hasFooName)
	
	state.SetString("Foo.Name", "Bob Dummy")
	data, _ := json.Marshal(dummy)
	fmt.Printf("dummy: %s\n", data)

	hasFoo, _ = state.Bool("Has.Foo")
	fmt.Printf("foo has marker: %v\n", 	hasFoo)
	hasFooName, _ = 	state.Bool("Foo.Has.Name")
	fmt.Printf("foo has marker: %v\n", 	hasFooName)
	
}

```

Additional notes

- Presence semantics:
  - If a struct has no set-marker field or the marker holder is nil, all fields are assumed present (IsSet returns true).
  - Extra fields in the main struct (without a corresponding marker bit) are allowed; their presence bit is simply not tracked.
  - Extra fields in the marker struct cause an error by default; use `WithNoStrict(true)` to ignore unmatched marker fields.
- Selectors and indexing:
  - Use `WithPathIndex(i)` to access slice items, for example: `state.SetString("Items.Name", "X", WithPathIndex(1))`.
  - On out-of-range index, `Set`/`SetValue` return an error; `Value` returns `nil`.
- Options:
  - `WithNoStrict(true)`: ignore marker fields that don’t exist on the main struct.
  - `WithIndex(map[string]int)`: provide a custom name→index mapping for marker fields.

Check unit tests for more advanced usage.

Gotchas

- Assume-present default: without a marker or with a nil marker holder, presence checks return true.
- Untracked fields: fields without corresponding marker bits are allowed but not presence-tracked.
- Strict by default: extra fields in the marker cause an error unless `WithNoStrict(true)` is used.
- Indexing: setters (`Set`/`SetValue`) return error on out-of-range; `Value` returns nil.
- Performance: this library uses reflection/unsafe—reuse `StateType`/`State` and avoid rebuilding selectors in hot paths.

## Why Another JSON Engine

`structology/encoding/json` exists because we need semantics that standard JSON libraries do not provide as a first-class runtime concern:

- Presence markers are the core requirement: update `setMarker:"true"` holders while decoding so code can distinguish `unset` vs `set to zero` vs `set to null`.
- Patch safety by design: partial update payloads must not accidentally overwrite fields just because a type has a zero value.
- Marker-aware compatibility: marshal/unmarshal must preserve marker behavior across nested structs, inline fields, and aliases.
- Datly parity: preserve existing datly behavior for tags (`json`, `jsonx`), case formatting, inline embedding, and compatibility edge cases.
- Low-allocation compiled execution: compile per-type marshal/unmarshal plans once, then execute with `xunsafe` accessors on hot paths.
- Context-aware extension points: keep path-aware marshal/unmarshal hooks and custom codec callbacks with context propagation.

The goal is not to replace `encoding/json` universally. The goal is marker-correct behavior for patch/update workloads first, and competitive performance for those shapes second.

Recent JSON runtime updates:
- Presence-marker robustness matrix added for alias/case-insensitive keys and `jsonx:"inline"` embedded paths.
- Custom `UnmarshalJSON` path now uses raw JSON value spans directly in fast struct decode (no parsed->marshal round-trip there).
- Typed container reuse expanded for unmarshal (`[]int`, `[]int64`, `[]float64`, `[]bool`, plus pointer variants), in addition to `[]string` and `map[string]string`.
- `WithFormatTag(&format.Tag{...})` support added: global case-format mapping and time/date layout control for marshal/unmarshal (for example `CaseFormat`, `DateFormat`, `TimeLayout`).

## JSON Benchmarks

`structology/encoding/json` benchmark comparison against Go standard library (`encoding/json`), run on `darwin/arm64` (Apple M1 Max) on `2026-02-24`.

Command:

```bash
go test ./encoding/json -run '^$' -bench 'BenchmarkCompare_(Marshal|Unmarshal)_(Basic|Advanced)_(Structology|Stdlib)$' -benchmem -benchtime=2s -count=1
go test ./encoding/json/marshal -run '^$' -bench 'BenchmarkEngine_MarshalPtrTo_ReuseBuffer|BenchmarkStdlib_MarshalPtr' -benchmem -benchtime=2s -count=1
```

Results:

| Benchmark | Structology | Stdlib |
|---|---:|---:|
| Marshal Basic | `87.63 ns/op`, `128 B/op`, `1 allocs/op` | `109.7 ns/op`, `48 B/op`, `1 allocs/op` |
| Unmarshal Basic | `483.3 ns/op`, `128 B/op`, `13 allocs/op` | `519.4 ns/op`, `256 B/op`, `6 allocs/op` |
| Marshal Advanced | `481.0 ns/op`, `560 B/op`, `6 allocs/op` | `655.6 ns/op`, `368 B/op`, `7 allocs/op` |
| Unmarshal Advanced | `1406 ns/op`, `744 B/op`, `35 allocs/op` | `2120 ns/op`, `888 B/op`, `21 allocs/op` |
| Marshal Ptr Reuse Buffer | `60.60 ns/op`, `0 B/op`, `0 allocs/op` | `109.6 ns/op`, `48 B/op`, `1 allocs/op` |

Notes:
- Marshal path is faster than stdlib on these basic/advanced cases.
- Unmarshal path remains faster than stdlib on these benchmark shapes.
- Correctness fixes for escaped-string decoding and input-buffer alias safety increased unmarshal allocations; next optimization focus is reducing those extra allocs without relaxing semantics.




## Contributing to structology

structology is an open source project and contributors are welcome!

Contributions are welcome via issues and pull requests.

## License

The source code is made available under the terms of the Apache License, Version 2, as stated in the file `LICENSE`.

Individual files may be made available under their own specific license,
all compatible with Apache License, Version 2. Please see individual files for details.



## Credits and Acknowledgements

**Library Author:** Adrian Witas
