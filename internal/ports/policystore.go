package ports

import "context"

// PolicyStore is the source of raw policy bytes. Implementations may back
// onto a file, a key-value store, a config service, or test fixtures. The
// store does not parse; the Loader owns parsing and validation.
//
// Contract:
//   - Load returns the entire policy as bytes. Concurrent calls are safe.
//   - Watch is optional. Implementations that cannot emit change
//     notifications return a nil channel and a nil error; callers that
//     receive a nil channel skip hot reload.
//   - When Watch is supported, the channel emits an empty struct each time
//     the underlying source changes. The Loader debounces and re-Loads.
//   - When ctx is cancelled, the Watch implementation closes the channel
//     and stops emitting.
//
// See docs/auth-design.md §4.3.
type PolicyStore interface {
	Load(ctx context.Context) ([]byte, error)
	Watch(ctx context.Context) (<-chan struct{}, error)
}
