package gpu

import "github.com/egoist/mygo/internal/scene"

// Renderer consumes a borrowed complete scene with the surface target current.
// Release runs once on that same UI executor before the target is destroyed.
type Renderer interface {
	Render(*scene.Scene) error
	Release()
}

// RendererFactory is an optional surface capability. Its drawing target and
// graphics API are owned by the external backend; Native may return zero.
type RendererFactory interface{ NewRenderer() (Renderer, error) }
