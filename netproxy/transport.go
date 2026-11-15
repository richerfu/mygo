package netproxy

import (
	"net/http"
	"sync"
)

type transportBinding struct {
	wrap func(*http.Transport) http.RoundTripper
}

var externalTransport struct {
	sync.RWMutex
	current *transportBinding
}

// UseTransport installs an external host's transport wrapper. The host can
// preserve ordered proxy failover that a single URL resolver cannot express.
// The wrapper must not mutate base, and must be safe for concurrent requests.
// Clients select the binding at request time, including clients created before
// host startup. In-flight requests keep their transport. The returned function
// removes only this registration and may be called repeatedly.
func UseTransport(wrap func(*http.Transport) http.RoundTripper) func() {
	b := &transportBinding{wrap}
	externalTransport.Lock()
	externalTransport.current = b
	externalTransport.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			externalTransport.Lock()
			defer externalTransport.Unlock()
			if externalTransport.current == b {
				externalTransport.current = nil
			}
		})
	}
}

// WrapTransport makes a client follow the current host wrapper, using base
// when no host is registered. Configure base first. Close idle connections through
// http.Client.CloseIdleConnections. A nil base uses the default system transport.
func WrapTransport(base *http.Transport) http.RoundTripper {
	if base == nil {
		base = Transport()
	}
	return &hostTransport{base: base}
}

// A client may be constructed by a package-level plugin before the host is
// installed. Select the current binding when making requests, caching one
// transport per binding so its connection pools are reused.
type hostTransport struct {
	base      *http.Transport
	mu        sync.Mutex
	binding   *transportBinding
	transport http.RoundTripper
}

func (t *hostTransport) selected() http.RoundTripper {
	externalTransport.RLock()
	binding := externalTransport.current
	externalTransport.RUnlock()
	t.mu.Lock()
	if t.transport != nil && t.binding == binding {
		current := t.transport
		t.mu.Unlock()
		return current
	}
	previous := t.transport
	current := http.RoundTripper(t.base)
	if binding != nil && binding.wrap != nil {
		current = binding.wrap(t.base)
	}
	t.binding, t.transport = binding, current
	t.mu.Unlock()
	if previous != nil {
		if c, ok := previous.(interface{ CloseIdleConnections() }); ok {
			c.CloseIdleConnections()
		}
	}
	return current
}
func (t *hostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.selected().RoundTrip(r)
}
func (t *hostTransport) CloseIdleConnections() {
	t.mu.Lock()
	current := t.transport
	t.mu.Unlock()
	if current != nil {
		if c, ok := current.(interface{ CloseIdleConnections() }); ok {
			c.CloseIdleConnections()
		}
	} else {
		t.base.CloseIdleConnections()
	}
}
