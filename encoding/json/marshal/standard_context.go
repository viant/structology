package marshal

import "context"

// MarshalContext checks cancellation before and after the existing standard
// encoder. Its synchronous encoding/json and custom codec calls cannot be
// interrupted mid-call. A native encoding error remains the primary error.
func (e *StandardEncoder) MarshalContext(ctx context.Context, value any) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	data, err := e.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return data, nil
}
