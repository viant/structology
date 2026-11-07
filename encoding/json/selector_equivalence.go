package json

import (
	"time"
)

// Only default policies are equivalent to the original bool-selector decoder.
// Explicit policies and user hooks retain native execution even when tags add
// alternate member spellings.
func standardSelectorOptions(cfg Options) bool {
	if cfg.setMode || cfg.setUnknownFieldPolicy || cfg.setNumberPolicy || cfg.setNullPolicy || cfg.setDuplicateKeyPolicy || cfg.setMalformedPolicy || cfg.setPathTracking || cfg.setCaseFormat {
		return false
	}
	if cfg.FormatTag != nil || cfg.TimeLayout != time.RFC3339 || cfg.TrackPresence || cfg.PathName != nil || cfg.PathExcluder != nil || cfg.DebugPathSink != nil || cfg.FieldUnmarshalHook != nil || cfg.PathUnmarshalHook != nil {
		return false
	}
	if _, ok := cfg.NameTransformer.(defaultNameTransformer); !ok {
		return false
	}
	if _, ok := cfg.FieldExcluder.(noExcluder); !ok {
		return false
	}
	_, builtin := cfg.scannerHooks.(scalarScannerHooks)
	return builtin
}
