package netproxy

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestHostResolverOwnershipAndPrecedence(t *testing.T) {
	clearEnv(t)
	first := UseResolver(func(*url.URL) (*url.URL, error) { t.Fatal("replaced resolver called"); return nil, nil })
	want := errors.New("host cannot apply PAC")
	calls := 0
	second := UseResolver(func(u *url.URL) (*url.URL, error) {
		calls++
		if u.Path != "/resource" {
			t.Fatalf("lost request path: %s", u)
		}
		return nil, want
	})
	t.Cleanup(second)
	first()
	first()
	req, _ := http.NewRequest("GET", "https://example.com/resource", nil)
	if _, err := Proxy(req); !errors.Is(err, want) {
		t.Fatalf("resolver error = %v", err)
	}
	local, _ := http.NewRequest("GET", "http://localhost/resource", nil)
	if p, err := Proxy(local); p != nil || err != nil {
		t.Fatalf("loopback = %v, %v", p, err)
	}
	saved := envProxy
	t.Cleanup(func() { envProxy = saved })
	envProxy = func(*http.Request) (*url.URL, error) { return &url.URL{Host: "env:80"}, nil }
	t.Setenv("HTTPS_PROXY", "http://env:80")
	if p, err := Proxy(req); err != nil || p.Host != "env:80" {
		t.Fatalf("environment = %v, %v", p, err)
	}
	if calls != 1 {
		t.Fatalf("resolver called %d times", calls)
	}
	second()
	if hostResolver() != nil {
		t.Fatal("resolver retained after release")
	}
}
