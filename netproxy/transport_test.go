package netproxy

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type countingTransport struct{ calls, closed atomic.Int32 }

func (t *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
}
func (t *countingTransport) CloseIdleConnections() { t.closed.Add(1) }

func TestExternalTransportLateRegistrationAndOwnership(t *testing.T) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	fallback := new(countingTransport)
	base.RegisterProtocol("test", fallback)
	client := &http.Client{Transport: WrapTransport(base)}
	request := func() {
		r, err := client.Get("test://host/value")
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
	}
	request()
	first := UseTransport(func(*http.Transport) http.RoundTripper { t.Fatal("replaced wrapper"); return nil })
	wrapped := new(countingTransport)
	factories := 0
	second := UseTransport(func(got *http.Transport) http.RoundTripper {
		if got != base {
			t.Fatal("lost base")
		}
		factories++
		return wrapped
	})
	defer second()
	first()
	first()
	request()
	request()
	if fallback.calls.Load() != 1 || wrapped.calls.Load() != 2 || factories != 1 {
		t.Fatal("early client missed host binding or did not reuse transport", fallback.calls.Load(), wrapped.calls.Load(), factories)
	}
	client.CloseIdleConnections()
	if wrapped.closed.Load() != 1 {
		t.Fatal("idle close not forwarded")
	}
	second()
	second()
	request()
	if fallback.calls.Load() != 2 || wrapped.calls.Load() != 2 || wrapped.closed.Load() != 2 {
		t.Fatal("released binding retained", fallback.calls.Load(), wrapped.calls.Load(), wrapped.closed.Load())
	}
}
