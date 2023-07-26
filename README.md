# structology - State tracker for golang struct 
[![GoReportCard](https://goreportcard.com/badge/github.com/viant/structology)](https://goreportcard.com/report/github.com/viant/structology)
[![GoDoc](https://godoc.org/github.com/viant/structology?status.svg)](https://godoc.org/github.com/viant/structology)

This library is compatible with Go 1.17+

Please refer to [`CHANGELOG.md`](CHANGELOG.md) if you encounter breaking changes.

- [Motivation](#motivation)
- [Usage](#usage)
- [Contribution](#contributing-to-godiff)
- [License](#license)

## Motivation


This project defines struct field set marker to distinctively identify input state, specifically for 'empty', 'nil', 'has been set' state.
This is critical for update/patch operation with large struct where only small subset if defined, 
with presence marker allowing you handling user input effectively, ensuring data integrity, and improving the security of applications  

Initially we have a few project holding marker abstraction to finally move it to this project.  

This project also implement a state that wraps arbitrary go struct, and provide selectors
to dynamically access/mutate any nested/addressable values, if set marker is defined all set operation also
would flag respective marker field 

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

	....// load entity and set all present fields with marker.Set(...)

	
    marker, err := NewMarker()
    if err != nil {
        log.Fatal(err)
    }
    isIdSet := marker.IsSet(marker.Index("Id"))
	fmt.Printf("is ID set : %v\n", isIdSet)
	
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

Check unit test for more advanced usage.
