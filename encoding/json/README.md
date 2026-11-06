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
