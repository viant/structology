package visitor

// Visitor is an interface that Visits over pairs of (key, element).
// The Visit method calls the provided callback for each pair.
// If the callback returns (false, nil), the Visit stops.
// If the callback returns an error, the Visit stops and returns that error.
type Visitor[K comparable, E any] func(func(key K, element E) (bool, error)) error
