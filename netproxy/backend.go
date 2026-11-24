package netproxy

import (
	"net/url"
	"sync"
)

type resolverBinding struct {
	resolve func(*url.URL) (*url.URL, error)
}

var externalResolver struct {
	sync.RWMutex
	current *resolverBinding
}

// UseResolver installs an external host's system proxy resolver. Loopback and
// explicit environment settings keep their normal precedence. The host owns
// caching and may return an error when it cannot apply a configured policy.
// The returned function removes only this binding; it is safe to call again.
// Requests already using the resolver may finish after it is removed.
func UseResolver(resolve func(*url.URL) (*url.URL, error)) func() {
	b := &resolverBinding{resolve: resolve}
	externalResolver.Lock()
	externalResolver.current = b
	externalResolver.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			externalResolver.Lock()
			if externalResolver.current == b {
				externalResolver.current = nil
			}
			externalResolver.Unlock()
		})
	}
}

func hostResolver() func(*url.URL) (*url.URL, error) {
	externalResolver.RLock()
	defer externalResolver.RUnlock()
	if b := externalResolver.current; b != nil {
		return b.resolve
	}
	return nil
}
