# structology - State tracker for golang struct 
[![GoReportCard](https://goreportcard.com/badge/github.com/viant/structology)](https://goreportcard.com/report/github.com/viant/structology)
[![GoDoc](https://godoc.org/github.com/viant/structology?status.svg)](https://godoc.org/github.com/viant/structology)

This library is compatible with Go 1.23+

- [Motivation](#motivation)
- [Usage](#usage)
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




## Contributing to structology

structology is an open source project and contributors are welcome!

Contributions are welcome via issues and pull requests.

## License

The source code is made available under the terms of the Apache License, Version 2, as stated in the file `LICENSE`.

Individual files may be made available under their own specific license,
all compatible with Apache License, Version 2. Please see individual files for details.



## Credits and Acknowledgements

**Library Author:** Adrian Witas
