package gpu

import "github.com/egoist/mygo/internal/scene"

// Renderer consumes a borrowed complete scene with the surface target current.
// Release runs once on that same UI executor before the target is destroyed.
type Renderer interface {
	Render(*scene.Scene) error
	Release()
}

// Capturer is an optional renderer or surface capability. Capture runs on the
// UI thread and consumes a borrowed complete scene; the host must make its
// drawing target current if needed. It returns owned, tightly packed,
// premultiplied RGBA pixels in top-to-bottom rows at the returned pixel size.
// An error or invalid buffer makes the UI fall back to software rendering.
type Capturer interface {
	Capture(s *scene.Scene) (width, height int, rgba []byte, err error)
}

// RendererFactory is an optional surface capability. Its drawing target and
// graphics API are owned by the external backend; Native may return zero.
type RendererFactory interface{ NewRenderer() (Renderer, error) }
