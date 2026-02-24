package jsontab

import (
	"context"

	"github.com/viant/tagly/format/text"
)

// Option mutates runtime options.
type Option interface{ apply(*Options) }

// Mode controls compatibility vs strict behavior.
type Mode int

const (
	ModeCompat Mode = iota
	ModeStrict
)

type UnknownHeaderPolicy int

const (
	IgnoreUnknownHeader UnknownHeaderPolicy = iota
	ErrorOnUnknownHeader
)

type ArityPolicy int

const (
	AllowArityMismatch ArityPolicy = iota
	ErrorOnArityMismatch
)

type MalformedPolicy int

const (
	TolerantMalformed MalformedPolicy = iota
	ErrorOnMalformed
)

// Options define jsontab runtime behavior.
type Options struct {
	Ctx                 context.Context
	Mode                Mode
	TagName             string
	CaseFormat          text.CaseFormat
	TimeLayout          string
	UnknownHeaderPolicy UnknownHeaderPolicy
	ArityPolicy         ArityPolicy
	MalformedPolicy     MalformedPolicy

	setTagName    bool
	setCaseFormat bool
	setTimeLayout bool
	setMode       bool
	setUnknown    bool
	setArity      bool
	setMalformed  bool
}
