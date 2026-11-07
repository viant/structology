# Effective JSON contracts

`WithRepresentation(metadata)` opts the native and standard JSON engines into
immutable `github.com/viant/x/shape.Representation` metadata. The value and decoder
destination retain their linked Go types, layouts, embedded fields, and methods.
Unchanged metadata retains ordinary linked-type behavior.

`MarshalStandard` and `UnmarshalStandard` retain `encoding/json` selection and
value semantics. They accept contract metadata and the `choice`, `enumAccept`,
and `enumStyle` annotations. Formatting transforms belong to the native engine.

Standard decoding does not normally maintain Datly presence holders. Applications
that require sparse-update tracking can opt in explicitly:

```go
err := json.UnmarshalStandard(data, &input,
    json.WithRepresentation(metadata),
    json.WithPresenceTracking(true),
)
```

Presence tracking sets canonical bool fields in a `setMarker:"true"` holder,
including explicitly supplied nulls and zero values. Public field renaming does
not rename the linked marker; hidden and absent input fields do not set it.
Promoted embedded fields and nested collections retain their own marker storage.

Custom `MarshalJSON`, `UnmarshalJSON`, and text methods own their complete wire
representation. Contract metadata does not override their internal fields. Hide
the enclosing field or adjust an ordinary wrapper when that is the intended policy.

## Variant plan caches

Native and standard encoders/decoders key effective field plans by the original
Go type and immutable representation identity. Alternating two policies for the
same Go type cannot overwrite one another's field names, visibility or enum plans.
An unchanged representation uses the original linked-type entry.

Shared plan caches are bounded to 2,048 entries and coalesce concurrent misses.
Native encoder plans with custom compile-time naming remain scoped to their
configured engine; representation clones share that engine's cache. Runtime
exclusion/name-transform callbacks remain on the current encoder and are not stored
in shared plans. No request values or authorization state enter these caches.

Datly's named contract cache supplies the same immutable representation for repeat
selections. Reuse that representation (or a configured marshaller) for cheap cache
hits. Independently rebuilt, equivalent representation objects are distinct keys;
codecs do not rescan/fingerprint the complete schema on every call. Cache eviction
can cause recompilation without changing behavior. Selecting nil/unchanged metadata
on a variant engine restores its linked contract rather than retaining the old variant.
