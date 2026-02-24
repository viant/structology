package jsontab

import (
	"context"
	"time"

	"github.com/viant/tagly/format/text"
)

type optionFn func(*Options)

func (o optionFn) apply(opts *Options) { o(opts) }

func WithContext(ctx context.Context) Option {
	return optionFn(func(o *Options) { o.Ctx = ctx })
}

func WithMode(mode Mode) Option {
	return optionFn(func(o *Options) {
		o.Mode = mode
		o.setMode = true
	})
}

func WithUnknownHeaderPolicy(policy UnknownHeaderPolicy) Option {
	return optionFn(func(o *Options) {
		o.UnknownHeaderPolicy = policy
		o.setUnknown = true
	})
}

func WithArityPolicy(policy ArityPolicy) Option {
	return optionFn(func(o *Options) {
		o.ArityPolicy = policy
		o.setArity = true
	})
}

func WithMalformedPolicy(policy MalformedPolicy) Option {
	return optionFn(func(o *Options) {
		o.MalformedPolicy = policy
		o.setMalformed = true
	})
}

func WithTagName(name string) Option {
	return optionFn(func(o *Options) {
		o.TagName = name
		o.setTagName = true
	})
}

func WithCaseFormat(caseFormat text.CaseFormat) Option {
	return optionFn(func(o *Options) {
		o.CaseFormat = caseFormat
		o.setCaseFormat = true
	})
}

func WithTimeLayout(layout string) Option {
	return optionFn(func(o *Options) {
		o.TimeLayout = layout
		o.setTimeLayout = true
	})
}

func defaultOptions() Options {
	return Options{
		Mode:                ModeCompat,
		TagName:             "csvName",
		CaseFormat:          text.CaseFormatUndefined,
		TimeLayout:          time.RFC3339,
		UnknownHeaderPolicy: IgnoreUnknownHeader,
		ArityPolicy:         AllowArityMismatch,
		MalformedPolicy:     TolerantMalformed,
	}
}

func resolveOptions(ctx context.Context, opts []Option) Options {
	result := defaultOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(&result)
	}
	if ctx != nil {
		result.Ctx = ctx
	}
	if result.Ctx == nil {
		result.Ctx = context.Background()
	}
	if result.TagName == "" {
		result.TagName = "csvName"
	}
	if result.TimeLayout == "" {
		result.TimeLayout = time.RFC3339
	}
	if result.Mode == ModeStrict {
		if !result.setUnknown {
			result.UnknownHeaderPolicy = ErrorOnUnknownHeader
		}
		if !result.setArity {
			result.ArityPolicy = ErrorOnArityMismatch
		}
		if !result.setMalformed {
			result.MalformedPolicy = ErrorOnMalformed
		}
	}
	return result
}
