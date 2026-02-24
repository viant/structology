package json

import (
	"context"
	"time"
	"unsafe"

	"github.com/viant/tagly/format"
	"github.com/viant/tagly/format/text"
	ftime "github.com/viant/tagly/format/time"
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

func WithUnknownFieldPolicy(policy UnknownFieldPolicy) Option {
	return optionFn(func(o *Options) {
		o.UnknownFieldPolicy = policy
		o.setUnknownFieldPolicy = true
	})
}

func WithNumberPolicy(policy NumberPolicy) Option {
	return optionFn(func(o *Options) {
		o.NumberPolicy = policy
		o.setNumberPolicy = true
	})
}

func WithNullPolicy(policy NullPolicy) Option {
	return optionFn(func(o *Options) {
		o.NullPolicy = policy
		o.setNullPolicy = true
	})
}

func WithDuplicateKeyPolicy(policy DuplicateKeyPolicy) Option {
	return optionFn(func(o *Options) {
		o.DuplicateKeyPolicy = policy
		o.setDuplicateKeyPolicy = true
	})
}

func WithMalformedPolicy(policy MalformedPolicy) Option {
	return optionFn(func(o *Options) {
		o.MalformedPolicy = policy
		o.setMalformedPolicy = true
	})
}

func WithPathTracking(mode PathTrackingMode) Option {
	return optionFn(func(o *Options) {
		o.PathTracking = mode
		o.setPathTracking = true
	})
}

func WithCaseFormat(caseFormat text.CaseFormat) Option {
	return optionFn(func(o *Options) {
		o.CaseFormat = caseFormat
		o.setCaseFormat = true
	})
}

func WithFormatTag(tag *format.Tag) Option {
	return optionFn(func(o *Options) { o.FormatTag = tag })
}

func WithNameTransformer(transformer NameTransformer) Option {
	return optionFn(func(o *Options) { o.NameTransformer = transformer })
}

func WithFieldExcluder(excluder FieldExcluder) Option {
	return optionFn(func(o *Options) { o.FieldExcluder = excluder })
}

func WithPathNameTransformer(transformer PathNameTransformer) Option {
	return optionFn(func(o *Options) { o.PathName = transformer })
}

func WithPathFieldExcluder(excluder PathFieldExcluder) Option {
	return optionFn(func(o *Options) { o.PathExcluder = excluder })
}

func WithOmitEmpty(enabled bool) Option {
	return optionFn(func(o *Options) {
		o.OmitEmpty = enabled
	})
}

func WithNilSlicePolicy(policy NilSlicePolicy) Option {
	return optionFn(func(o *Options) {
		o.NilSlicePolicy = policy
	})
}

func WithDebugPathSink(sink func(PathRef)) Option {
	return optionFn(func(o *Options) { o.DebugPathSink = sink })
}

func WithFieldUnmarshalHook(hook func(ctx context.Context, holder unsafe.Pointer, field string, value any) (any, error)) Option {
	return optionFn(func(o *Options) { o.FieldUnmarshalHook = hook })
}

func WithPathUnmarshalHook(hook func(ctx context.Context, holder unsafe.Pointer, path []string, field string, value any) (any, error)) Option {
	return optionFn(func(o *Options) { o.PathUnmarshalHook = hook })
}

func WithScannerHooks(hooks ScannerHooks) Option {
	return optionFn(func(o *Options) { o.scannerHooks = hooks })
}

func defaultOptions() Options {
	return Options{
		Mode:               ModeCompat,
		UnknownFieldPolicy: IgnoreUnknown,
		NumberPolicy:       CoerceNumbers,
		NullPolicy:         CompatNulls,
		DuplicateKeyPolicy: LastWins,
		MalformedPolicy:    Tolerant,
		PathTracking:       PathTrackingErrorsOnly,
		CaseFormat:         text.CaseFormatUndefined,
		TimeLayout:         time.RFC3339,
		NameTransformer:    defaultNameTransformer{},
		FieldExcluder:      noExcluder{},
		scannerHooks:       scalarScannerHooks{},
		OmitEmpty:          false,
		NilSlicePolicy:     NilSliceAsNull,
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
	if result.FormatTag != nil {
		if result.TimeLayout == "" {
			result.TimeLayout = time.RFC3339
		}
		if result.FormatTag.TimeLayout != "" {
			result.TimeLayout = result.FormatTag.TimeLayout
		} else if result.FormatTag.DateFormat != "" {
			result.TimeLayout = ftime.DateFormatToTimeLayout(result.FormatTag.DateFormat)
		}
		if !result.setCaseFormat && result.CaseFormat == text.CaseFormatUndefined {
			cf := text.CaseFormat(result.FormatTag.CaseFormat)
			if cf != "" && cf != "-" {
				result.CaseFormat = cf
				result.setCaseFormat = true
			}
		}
	}
	if result.FieldUnmarshalHook != nil {
		fieldHook := result.FieldUnmarshalHook
		if result.PathUnmarshalHook == nil {
			result.PathUnmarshalHook = func(ctx context.Context, holder unsafe.Pointer, _ []string, field string, value any) (any, error) {
				return fieldHook(ctx, holder, field, value)
			}
		} else {
			pathHook := result.PathUnmarshalHook
			result.PathUnmarshalHook = func(ctx context.Context, holder unsafe.Pointer, path []string, field string, value any) (any, error) {
				transformed, err := pathHook(ctx, holder, path, field, value)
				if err != nil {
					return nil, err
				}
				return fieldHook(ctx, holder, field, transformed)
			}
		}
	}

	if result.Mode == ModeStrict {
		if !result.setUnknownFieldPolicy {
			result.UnknownFieldPolicy = ErrorOnUnknown
		}
		if !result.setNumberPolicy {
			result.NumberPolicy = ExactNumbers
		}
		if !result.setNullPolicy {
			result.NullPolicy = StrictNulls
		}
		if !result.setDuplicateKeyPolicy {
			result.DuplicateKeyPolicy = ErrorOnDuplicate
		}
		if !result.setMalformedPolicy {
			result.MalformedPolicy = FailFast
		}
	}
	if result.setCaseFormat {
		if _, ok := result.NameTransformer.(defaultNameTransformer); ok {
			result.NameTransformer = caseFormatTransformer{caseFormat: result.CaseFormat}
		}
	}
	return result
}
