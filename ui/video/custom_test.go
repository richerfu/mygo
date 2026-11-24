//go:build mygo_custom_backend

package video

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/backend/media"
)

func TestCustomFactorySources(t *testing.T) {
	want := errors.New("host rejected the source")
	var source media.Source
	release := media.UseFactory(func(s media.Source) (media.Engine, error) {
		source = s
		return nil, want
	})
	t.Cleanup(release)
	if _, err := OpenData([]byte{1, 2, 3}); !errors.Is(err, want) || len(source.Data) != 3 || source.URL != "" {
		t.Fatalf("memory source: %+v, %v", source, err)
	}
	path := filepath.Join(t.TempDir(), "视频 clip.mp4")
	if _, err := Open(path); !errors.Is(err, want) || source.Path != path || source.URL != fileURL(path) || source.Data != nil {
		t.Fatalf("file source: %+v, %v", source, err)
	}
	const url = "https://example.invalid/video.mp4?name=a%20b"
	if _, err := OpenURL(url); !errors.Is(err, want) || source.URL != url || source.Path != "" {
		t.Fatalf("URL source: %+v, %v", source, err)
	}
	release()
	if _, err := OpenURL(url); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("released host: %v", err)
	}
}
