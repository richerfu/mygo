package media

import (
	"errors"
	"testing"
)

func TestFactoryReplacementDoesNotRestoreDisposedHost(t *testing.T) {
	first := UseFactory(func(Source) (Engine, error) { t.Fatal("disposed host called"); return nil, nil })
	want := errors.New("native source failed")
	second := UseFactory(func(s Source) (Engine, error) {
		if s.URL != "https://example.test/movie" || len(s.Data) != 3 {
			t.Fatalf("source lost: %+v", s)
		}
		return nil, want
	})
	t.Cleanup(second)
	first()
	first()
	if _, err := Factory()(Source{URL: "https://example.test/movie", Data: []byte{1, 2, 3}}); !errors.Is(err, want) {
		t.Fatal(err)
	}
	second()
	if Factory() != nil {
		t.Fatal("released host retained")
	}
}
