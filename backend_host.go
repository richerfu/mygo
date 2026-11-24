package mygo

import (
	"errors"
	nativebackend "github.com/egoist/mygo/backend"
)

// UseBackend installs a native backend before any API first uses the platform.
// backend.Backend is the public contract. A process owns one backend for its
// entire lifetime. Installation after initialization returns an error.
func UseBackend(b nativebackend.Backend) error {
	if b == nil {
		return errors.New("mygo: nil native backend")
	}
	installed := false
	backendOnce.Do(func() { theBackend = b; installed = true })
	if !installed {
		return errors.New("mygo: native backend already initialized")
	}
	return nil
}
