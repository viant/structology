package jsontab

import (
	"context"
	"strings"

	jsmarshal "github.com/viant/structology/encoding/jsontab/marshal"
	jsunmarshal "github.com/viant/structology/encoding/jsontab/unmarshal"
)

var (
	defaultMarshalEngine   = jsmarshal.New("csvName", "", nil, "")
	defaultUnmarshalEngine = jsunmarshal.New(
		"csvName",
		"",
		nil,
		"",
		jsunmarshal.IgnoreUnknownHeader,
		jsunmarshal.AllowArityMismatch,
		jsunmarshal.TolerantMalformed,
	)
)

func MarshalContext(ctx context.Context, value interface{}, opts ...Option) ([]byte, error) {
	if len(opts) == 0 {
		return defaultMarshalEngine.Marshal(value)
	}
	cfg := resolveOptions(ctx, opts)
	caseKey := ""
	var compileName func(string) string
	if cfg.CaseFormat != "" {
		tr := caseFormatTransformer{caseFormat: cfg.CaseFormat}
		caseKey = string(cfg.CaseFormat)
		compileName = func(field string) string { return tr.Transform(field) }
	}
	m := jsmarshal.New(cfg.TagName, caseKey, compileName, cfg.TimeLayout)
	return m.Marshal(value)
}

func Marshal(value interface{}, opts ...Option) ([]byte, error) {
	return MarshalContext(context.Background(), value, opts...)
}

func UnmarshalContext(ctx context.Context, data []byte, dest interface{}, opts ...Option) error {
	if len(opts) == 0 {
		return defaultUnmarshalEngine.Unmarshal(data, dest)
	}
	cfg := resolveOptions(ctx, opts)
	caseKey := ""
	var compileName func(string) string
	if cfg.CaseFormat != "" {
		tr := caseFormatTransformer{caseFormat: cfg.CaseFormat}
		caseKey = string(cfg.CaseFormat)
		compileName = func(field string) string { return tr.Transform(field) }
	}
	unknown := jsunmarshal.IgnoreUnknownHeader
	if cfg.UnknownHeaderPolicy == ErrorOnUnknownHeader {
		unknown = jsunmarshal.ErrorOnUnknownHeader
	}
	arity := jsunmarshal.AllowArityMismatch
	if cfg.ArityPolicy == ErrorOnArityMismatch {
		arity = jsunmarshal.ErrorOnArityMismatch
	}
	malformed := jsunmarshal.TolerantMalformed
	if cfg.MalformedPolicy == ErrorOnMalformed {
		malformed = jsunmarshal.ErrorOnMalformed
	}
	u := jsunmarshal.New(cfg.TagName, caseKey, compileName, cfg.TimeLayout, unknown, arity, malformed)
	return u.Unmarshal(data, dest)
}

func Unmarshal(data []byte, dest interface{}, opts ...Option) error {
	return UnmarshalContext(context.Background(), data, dest, opts...)
}

func normalizeTagName(tagName string) string {
	if tagName == "" {
		return "csvName"
	}
	return strings.TrimSpace(tagName)
}
