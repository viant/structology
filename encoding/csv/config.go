package csv

import io "github.com/viant/structology/encoding/csv/internal/scalar"

type (
	//Config represents reader config
	Config struct {
		FieldSeparator  string
		ObjectSeparator string
		EncloseBy       string
		EscapeBy        string
		// RawNullValue emits the null marker as an already encoded wire token.
		// Literal strings equal to the marker remain enclosed and escaped.
		RawNullValue      bool
		NullValue         string
		Stringify         StringifyConfig
		UniqueFields      []string
		References        []*Reference // parent -> children. Foo.ID -> Boo.FooId
		ExcludedPaths     []string
		StringifierConfig io.StringifierConfig
	}

	// StringifyConfig "extends" Config with ignore flags
	StringifyConfig struct {
		IgnoreFieldSeparator  bool
		IgnoreObjectSeparator bool
		IgnoreEncloseBy       bool
	}
)
