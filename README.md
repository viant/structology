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

## Usage

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
        Has    *EntityHas `presence:"true"`
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

